package glustershd

import (
	"encoding/json"
	"encoding/xml"
	"net/http"

	"github.com/gluster/glusterd2/glusterd2/gdctx"
	restutils "github.com/gluster/glusterd2/glusterd2/servers/rest/utils"
	"github.com/gluster/glusterd2/glusterd2/transaction"
	"github.com/gluster/glusterd2/glusterd2/volume"
	"github.com/gluster/glusterd2/pkg/api"
	"github.com/gluster/glusterd2/pkg/errors"

	"github.com/gorilla/mux"
	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
)

func glustershEnableHandler(w http.ResponseWriter, r *http.Request) {
	// Implement the help logic and send response back as below
	p := mux.Vars(r)
	volname := p["name"]

	ctx := r.Context()
	logger := gdctx.GetReqLogger(ctx)

	//validate volume name
	v, err := volume.GetVolume(volname)
	if err != nil {
		restutils.SendHTTPError(ctx, w, http.StatusNotFound, errors.ErrVolNotFound.Error(), api.ErrCodeDefault)
		return
	}

	// validate volume type
	if v.Type != volume.Replicate && v.Type != volume.Disperse {
		restutils.SendHTTPError(ctx, w, http.StatusBadRequest, "Volume Type not supported", api.ErrCodeDefault)
		return
	}

	// Transaction which starts self heal daemon on all nodes with atleast one brick.
	txn := transaction.NewTxn(ctx)
	defer txn.Cleanup()

	//Lock on Volume Name
	lock, unlock, err := transaction.CreateLockSteps(volname)
	if err != nil {
		restutils.SendHTTPError(ctx, w, http.StatusInternalServerError, err.Error(), api.ErrCodeDefault)
		return
	}

	v.HealFlag = true

	txn.Nodes = v.Nodes()
	txn.Steps = []*transaction.Step{
		lock,
		{
			DoFunc: "vol-option.UpdateVolinfo",
			Nodes:  txn.Nodes,
		},
		{
			DoFunc: "selfheal-start.Commit",
			Nodes:  txn.Nodes,
		},
		unlock,
	}

	if err := txn.Ctx.Set("volinfo", v); err != nil {
		logger.WithError(err).Error("failed to set volinfo in transaction context")
		restutils.SendHTTPError(ctx, w, http.StatusInternalServerError, err.Error(), api.ErrCodeDefault)
		return
	}

	err = txn.Do()
	if err != nil {
		logger.WithFields(log.Fields{
			"error":   err.Error(),
			"volname": volname,
		}).Error("failed to start self heal daemon")
		restutils.SendHTTPError(ctx, w, http.StatusInternalServerError, err.Error(), api.ErrCodeDefault)
		return
	}

	restutils.SendHTTPResponse(ctx, w, http.StatusOK, "Glustershd Help")
}

func glustershDisableHandler(w http.ResponseWriter, r *http.Request) {
	p := mux.Vars(r)
	volname := p["name"]

	ctx := r.Context()
	logger := gdctx.GetReqLogger(ctx)

	//validate volume name
	v, err := volume.GetVolume(volname)
	if err != nil {
		restutils.SendHTTPError(ctx, w, http.StatusNotFound, errors.ErrVolNotFound.Error(), api.ErrCodeDefault)
		return
	}

	// validate volume type
	if v.Type != volume.Replicate && v.Type != volume.Disperse {
		restutils.SendHTTPError(ctx, w, http.StatusBadRequest, "Volume Type not supported", api.ErrCodeDefault)
		return
	}

	// Transaction which checks if all replicate volumes are stopped before
	// stopping the self-heal daemon.
	txn := transaction.NewTxn(ctx)
	defer txn.Cleanup()

	// Lock on volume name.
	lock, unlock, err := transaction.CreateLockSteps(volname)
	if err != nil {
		restutils.SendHTTPError(ctx, w, http.StatusInternalServerError, err.Error(), api.ErrCodeDefault)
		return
	}

	v.HealFlag = false

	txn.Nodes = v.Nodes()
	txn.Steps = []*transaction.Step{
		lock,
		{
			DoFunc: "vol-option.UpdateVolinfo",
			Nodes:  txn.Nodes,
		},

		{
			DoFunc: "selfheal-stop.Commit",
			Nodes:  txn.Nodes,
		},
		unlock,
	}

	if err := txn.Ctx.Set("volinfo", v); err != nil {
		logger.WithError(err).Error("failed to set volinfo in transaction context")
		restutils.SendHTTPError(ctx, w, http.StatusInternalServerError, err.Error(), api.ErrCodeDefault)
		return
	}

	err = txn.Do()
	if err != nil {
		logger.WithFields(log.Fields{
			"error":   err.Error(),
			"volname": volname,
		}).Error("failed to stop self heal daemon")
		restutils.SendHTTPError(ctx, w, http.StatusInternalServerError, err.Error(), api.ErrCodeDefault)
		return
	}

	restutils.SendHTTPResponse(ctx, w, http.StatusOK, "Glustershd Help")
}

func glustershInfoHandler(w http.ResponseWriter, r *http.Request) {
	p := mux.Vars(r)
	volname := p["name"]
	option := p["opts"]
	_ = option

	ctx := r.Context()
	logger := gdctx.GetReqLogger(ctx)

	//validate volume name
	v, err := volume.GetVolume(volname)
	if err != nil {
		restutils.SendHTTPError(ctx, w, http.StatusNotFound, errors.ErrVolNotFound.Error(), api.ErrCodeDefault)
		return
	}

	// validate volume type
	if v.Type != volume.Replicate && v.Type != volume.Disperse {
		restutils.SendHTTPError(ctx, w, http.StatusBadRequest, "Volume Type not supported", api.ErrCodeDefault)
		return
	}

	txn := transaction.NewTxn(ctx)
	defer txn.Cleanup()

	//Lock on Volume Name
	lock, unlock, err := transaction.CreateLockSteps(volname)
	if err != nil {
		restutils.SendHTTPError(ctx, w, http.StatusInternalServerError, err.Error(), api.ErrCodeDefault)
		return
	}

	txn.Steps = []*transaction.Step{
		lock,
		{
			DoFunc: "heal-info.Commit",
			Nodes:  []uuid.UUID{gdctx.MyUUID},
		},
		unlock,
	}

	if err := txn.Ctx.Set("volname", volname); err != nil {
		logger.WithError(err).Error("failed to set volname in transaction context")
		restutils.SendHTTPError(ctx, w, http.StatusInternalServerError, err.Error(), api.ErrCodeDefault)

		return
	}

	var options []string

	options = append(options, option, "xml")
	if err := txn.Ctx.Set("option", options); err != nil {
		logger.WithError(err).Error("failed to set volname in transaction context")
		restutils.SendHTTPError(ctx, w, http.StatusInternalServerError, err.Error(), api.ErrCodeDefault)
		return
	}

	err = txn.Do()
	if err != nil {
		logger.WithFields(log.Fields{
			"error":   err.Error(),
			"volname": volname,
		}).Error("failed to retrieve heal info")
		restutils.SendHTTPError(ctx, w, http.StatusInternalServerError, err.Error(), api.ErrCodeDefault)
		return
	}
	var out string
	err = txn.Ctx.GetNodeResult(gdctx.MyUUID, "stdout", &out)
	if err != nil {
		logger.WithFields(log.Fields{
			"error": err.Error(),
		}).Error("Could Not Fetch Stdout", err)

		return
	}
	output := []byte(out)

	var info HealInfo
	err = xml.Unmarshal(output, &info)
	if err != nil {

		logger.WithFields(log.Fields{
			"error": err.Error(),
		}).Error("Error unmarshalling from XML", err)

		return
	}

	JsonOutput, err := json.Marshal(&info.Bricks)
	if err != nil {

		logger.WithFields(log.Fields{
			"result": JsonOutput,
		}).Error("Error marshalling to JSON", err)

		return
	}

	restutils.SendHTTPResponse(ctx, w, http.StatusOK, string(JsonOutput))

}

func granularHealEnableHandler(w http.ResponseWriter, r *http.Request) {
	p := mux.Vars(r)
	volname := p["name"]

	ctx := r.Context()
	logger := gdctx.GetReqLogger(ctx)

	//validate volume name
	v, err := volume.GetVolume(volname)
	if err != nil {
		restutils.SendHTTPError(ctx, w, http.StatusNotFound, errors.ErrVolNotFound.Error(), api.ErrCodeDefault)
		return
	}

	// validate volume type
	if v.Type != volume.Replicate && v.Type != volume.Disperse {
		restutils.SendHTTPError(ctx, w, http.StatusBadRequest, "Volume Type not supported", api.ErrCodeDefault)
		return
	}

	txn := transaction.NewTxn(ctx)
	defer txn.Cleanup()

	//Lock on Volume Name
	lock, unlock, err := transaction.CreateLockSteps(volname)
	if err != nil {
		restutils.SendHTTPError(ctx, w, http.StatusInternalServerError, err.Error(), api.ErrCodeDefault)
		return
	}

	v.GranularHealEntry = true

	txn.Nodes = v.Nodes()
	txn.Steps = []*transaction.Step{
		lock,
		{
			DoFunc: "granular-heal.Enable",
			Nodes:  []uuid.UUID{gdctx.MyUUID},
		},
		{
			DoFunc: "vol-option.UpdateVolinfo",
			Nodes:  txn.Nodes,
		},
		unlock,
	}

	if err := txn.Ctx.Set("volinfo", v); err != nil {
		logger.WithError(err).Error("failed to set volinfo in transaction context")
		restutils.SendHTTPError(ctx, w, http.StatusInternalServerError, err.Error(), api.ErrCodeDefault)
		return
	}

	if err := txn.Ctx.Set("volname", volname); err != nil {
		logger.WithError(err).Error("failed to set volname in transaction context")
		restutils.SendHTTPError(ctx, w, http.StatusInternalServerError, err.Error(), api.ErrCodeDefault)
		return
	}

	var options []string

	options = append(options, "granular-entry-heal-op")
	if err := txn.Ctx.Set("option", options); err != nil {
		logger.WithError(err).Error("failed to set volname in transaction context")
		restutils.SendHTTPError(ctx, w, http.StatusInternalServerError, err.Error(), api.ErrCodeDefault)
		return
	}

	err = txn.Do()
	if err != nil {
		logger.WithFields(log.Fields{
			"error":   err.Error(),
			"volname": volname,
		}).Error("failed to enable Granular Entry Heal Option")
		restutils.SendHTTPError(ctx, w, http.StatusInternalServerError, err.Error(), api.ErrCodeDefault)
		return
	}

}

func granularHealDisableHandler(w http.ResponseWriter, r *http.Request) {
	p := mux.Vars(r)
	volname := p["name"]

	ctx := r.Context()
	logger := gdctx.GetReqLogger(ctx)

	//validate volume name
	v, err := volume.GetVolume(volname)
	if err != nil {
		restutils.SendHTTPError(ctx, w, http.StatusNotFound, errors.ErrVolNotFound.Error(), api.ErrCodeDefault)
		return
	}

	// validate volume type
	if v.Type != volume.Replicate && v.Type != volume.Disperse {
		restutils.SendHTTPError(ctx, w, http.StatusBadRequest, "Volume Type not supported", api.ErrCodeDefault)
		return
	}

	txn := transaction.NewTxn(ctx)
	defer txn.Cleanup()

	//Lock on Volume Name
	lock, unlock, err := transaction.CreateLockSteps(volname)
	if err != nil {
		restutils.SendHTTPError(ctx, w, http.StatusInternalServerError, err.Error(), api.ErrCodeDefault)
		return
	}

	v.GranularHealEntry = false

	txn.Nodes = v.Nodes()
	txn.Steps = []*transaction.Step{
		lock,
		{
			DoFunc: "vol-option.UpdateVolinfo",
			Nodes:  txn.Nodes,
		},
		unlock,
	}

	if err := txn.Ctx.Set("volinfo", v); err != nil {
		logger.WithError(err).Error("failed to set volinfo in transaction context")
		restutils.SendHTTPError(ctx, w, http.StatusInternalServerError, err.Error(), api.ErrCodeDefault)
		return
	}

	err = txn.Do()
	if err != nil {
		logger.WithFields(log.Fields{
			"error":   err.Error(),
			"volname": volname,
		}).Error("failed to disable Granular Entry Heal Option")
		restutils.SendHTTPError(ctx, w, http.StatusInternalServerError, err.Error(), api.ErrCodeDefault)
		return
	}

}
