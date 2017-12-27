package volumecommands

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gluster/glusterd2/glusterd2/brick"
	"github.com/gluster/glusterd2/glusterd2/events"
	"github.com/gluster/glusterd2/glusterd2/gdctx"
	restutils "github.com/gluster/glusterd2/glusterd2/servers/rest/utils"
	"github.com/gluster/glusterd2/glusterd2/transaction"
	"github.com/gluster/glusterd2/glusterd2/volume"
	"github.com/gluster/glusterd2/pkg/api"
	gderrors "github.com/gluster/glusterd2/pkg/errors"

	"github.com/pborman/uuid"
)

func unmarshalVolCreateRequest(msg *api.VolCreateReq, r *http.Request) (int, error) {
	if err := restutils.UnmarshalRequest(r, msg); err != nil {
		return 422, gderrors.ErrJSONParsingFailed
	}

	if msg.Name == "" {
		return http.StatusBadRequest, gderrors.ErrEmptyVolName
	}

	if len(msg.Subvols) <= 0 {
		return http.StatusBadRequest, gderrors.ErrEmptyBrickList
	}

	for _, subvol := range msg.Subvols {
		if len(subvol.Bricks) <= 0 {
			return http.StatusBadRequest, gderrors.ErrEmptyBrickList
		}
	}
	return 0, nil

}

func voltypeFromSubvols(req *api.VolCreateReq) volume.VolType {
	if len(req.Subvols) == 0 {
		return volume.Distribute
	}
	// TODO: Don't know how to decide on Volume Type if each subvol is different
	// For now just picking the first subvols Type, which satisfies
	// most of today's needs
	switch req.Subvols[0].Type {
	case "replicate":
		return volume.Replicate
	case "distribute":
		return volume.Distribute
	default:
		return volume.Distribute
	}
}

func createVolinfo(req *api.VolCreateReq) (*volume.Volinfo, error) {

	var err error

	v := new(volume.Volinfo)
	if req.Options != nil {
		v.Options = req.Options
	} else {
		v.Options = make(map[string]string)
	}
	v.ID = uuid.NewRandom()
	v.Name = req.Name

	if len(req.Transport) > 0 {
		v.Transport = req.Transport
	} else {
		v.Transport = "tcp"
	}

	v.DistCount = len(req.Subvols)

	v.Type = voltypeFromSubvols(req)

	for idx, subvolreq := range req.Subvols {
		if subvolreq.ReplicaCount == 0 && subvolreq.Type == "replicate" {
			return nil, errors.New("Replica count not specified")
		}

		if subvolreq.ReplicaCount > 0 && subvolreq.ReplicaCount != len(subvolreq.Bricks) {
			return nil, errors.New("Invalid number of bricks")
		}

		name := subvolreq.Name
		if name == "" {
			name = fmt.Sprintf("s-%d", idx)
		}

		ty := volume.SubvolDistribute
		switch subvolreq.Type {
		case "replicate":
			ty = volume.SubvolReplicate
		case "disperse":
			ty = volume.SubvolDisperse
		default:
			ty = volume.SubvolDistribute
		}

		s := volume.Subvol{
			Name: name,
			Type: ty,
		}

		if subvolreq.ArbiterCount != 0 {
			if subvolreq.ReplicaCount != 3 || subvolreq.ArbiterCount != 1 {
				return nil, errors.New("For arbiter configuration, replica count must be 3 and arbiter count must be 1. The 3rd brick of the replica will be the arbiter")
			}
			s.ArbiterCount = 1
		}

		if subvolreq.ReplicaCount == 0 {
			s.ReplicaCount = 1
		} else {
			s.ReplicaCount = subvolreq.ReplicaCount
		}
		s.Bricks, err = volume.NewBrickEntriesFunc(subvolreq.Bricks, v.Name, v.ID)
		if err != nil {
			return nil, err
		}
		v.Subvols = append(v.Subvols, s)
	}

	v.Auth = volume.VolAuth{
		Username: uuid.NewRandom().String(),
		Password: uuid.NewRandom().String(),
	}

	v.State = volume.VolCreated

	return v, nil
}

func validateVolumeCreate(c transaction.TxnCtx) error {

	var req api.VolCreateReq
	err := c.Get("req", &req)
	if err != nil {
		return err
	}

	var volinfo volume.Volinfo
	err = c.Get("volinfo", &volinfo)
	if err != nil {
		return err
	}

	var bricks []brick.Brickinfo
	for _, subvol := range volinfo.Subvols {
		for _, brick := range subvol.Bricks {
			bricks = append(bricks, brick)
		}
	}

	// FIXME: Return values of this function are inconsistent and unused
	if _, err = volume.ValidateBrickEntriesFunc(bricks, volinfo.ID, req.Force); err != nil {
		c.Logger().WithError(err).WithField(
			"volume", volinfo.Name).Debug("validateVolumeCreate: failed to validate bricks")
		return err
	}

	return nil
}

func rollBackVolumeCreate(c transaction.TxnCtx) error {

	var volinfo volume.Volinfo
	if err := c.Get("volinfo", &volinfo); err != nil {
		return err
	}

	for _, subvol := range volinfo.Subvols {
		for _, b := range subvol.Bricks {
			if !uuid.Equal(b.NodeID, gdctx.MyUUID) {
				continue
			}

			// TODO: Clean xattrs set if any. ValidateBrickEntriesFunc()
			// does a lot of things that it's not supposed to do.
		}
	}
	return nil
}

func registerVolCreateStepFuncs() {
	var sfs = []struct {
		name string
		sf   transaction.StepFunc
	}{
		{"vol-create.Validate", validateVolumeCreate},
		{"vol-create.StoreVolume", storeVolume},
		{"vol-create.Rollback", rollBackVolumeCreate},
	}
	for _, sf := range sfs {
		transaction.RegisterStepFunc(sf.sf, sf.name)
	}
}

func volumeCreateHandler(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()
	logger := gdctx.GetReqLogger(ctx)

	req := new(api.VolCreateReq)
	httpStatus, err := unmarshalVolCreateRequest(req, r)
	if err != nil {
		logger.WithError(err).Error("Failed to unmarshal volume request")
		restutils.SendHTTPError(ctx, w, httpStatus, err.Error(), api.ErrCodeDefault)
		return
	}

	if volume.ExistsFunc(req.Name) {
		restutils.SendHTTPError(ctx, w, http.StatusInternalServerError, gderrors.ErrVolExists.Error(), api.ErrCodeDefault)
		return
	}

	var nodesMap = make(map[string]int)
	var nodes []uuid.UUID
	for _, subvol := range req.Subvols {
		for _, brick := range subvol.Bricks {
			if _, ok := nodesMap[brick.NodeID]; !ok {
				nodesMap[brick.NodeID] = 1
				nodes = append(nodes, uuid.Parse(brick.NodeID))
			}
		}
	}

	if err := validateOptions(req.Options); err != nil {
		logger.WithField("option", err.Error()).Error("invalid volume option specified")
		msg := fmt.Sprintf("invalid volume option specified: %s", err.Error())
		restutils.SendHTTPError(ctx, w, http.StatusBadRequest, msg, api.ErrCodeDefault)
		return
	}

	txn := transaction.NewTxn(ctx)
	defer txn.Cleanup()

	lock, unlock, err := transaction.CreateLockSteps(req.Name)
	if err != nil {
		restutils.SendHTTPError(ctx, w, http.StatusInternalServerError, err.Error(), api.ErrCodeDefault)
		return
	}

	txn.Steps = []*transaction.Step{
		lock,
		{
			DoFunc: "vol-create.Validate",
			Nodes:  nodes,
		},
		{
			DoFunc: "vol-create.StoreVolume",
			Nodes:  []uuid.UUID{gdctx.MyUUID},
		},
		unlock,
	}

	err = txn.Ctx.Set("req", req)
	if err != nil {
		logger.WithError(err).Error("failed to set request in transaction context")
		restutils.SendHTTPError(ctx, w, http.StatusInternalServerError, err.Error(), api.ErrCodeDefault)
		return
	}

	vol, err := createVolinfo(req)
	if err != nil {
		logger.WithError(err).Error("failed to create volinfo")
		restutils.SendHTTPError(ctx, w, http.StatusInternalServerError, err.Error(), api.ErrCodeDefault)
		return
	}

	err = txn.Ctx.Set("volinfo", vol)
	if err != nil {
		logger.WithError(err).Error("failed to set volinfo in transaction context")
		restutils.SendHTTPError(ctx, w, http.StatusInternalServerError, err.Error(), api.ErrCodeDefault)
		return
	}

	err = txn.Do()
	if err != nil {
		logger.WithError(err).Error("volume create transaction failed")
		if err == transaction.ErrLockTimeout {
			restutils.SendHTTPError(ctx, w, http.StatusConflict, err.Error(), api.ErrCodeDefault)
		} else {
			restutils.SendHTTPError(ctx, w, http.StatusInternalServerError, err.Error(), api.ErrCodeDefault)
		}
		return
	}

	if err = txn.Ctx.Get("volinfo", &vol); err != nil {
		restutils.SendHTTPError(ctx, w, http.StatusInternalServerError, "failed to get volinfo", api.ErrCodeDefault)
		return
	}

	txn.Ctx.Logger().WithField("volname", vol.Name).Info("new volume created")
	events.Broadcast(newVolumeEvent(eventVolumeCreated, vol))

	resp := createVolumeCreateResp(vol)
	restutils.SendHTTPResponse(ctx, w, http.StatusCreated, resp)
}

func createVolumeCreateResp(v *volume.Volinfo) *api.VolumeCreateResp {
	return (*api.VolumeCreateResp)(createVolumeInfoResp(v))
}
