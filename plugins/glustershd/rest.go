package glustershd

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"os/exec"
	"strings"

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

func runGlfshealWithArgs(r *http.Request, volname string, option string) string {

	ctx := r.Context()
	logger := gdctx.GetReqLogger(ctx)

	var out bytes.Buffer
	var buffer bytes.Buffer

	buffer.WriteString(fmt.Sprintf("%s", volname))
	buffer.WriteString(fmt.Sprintf(" %s", option))
	buffer.WriteString(fmt.Sprintf(" xml"))

	a := buffer.String()
	args := strings.Fields(a)
	path, e := exec.LookPath("glfsheal")

	if e != nil {

		logger.WithFields(log.Fields{
			"volname": volname,
		}).Error("Error running glfsheal binary")

		return ""
	}

	cmd := exec.Command(path, args...)
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {

		logger.WithFields(log.Fields{
			"error": err.Error(),
		}).Error("Heal Info operation failed.")

		return ""
	}
	output := []byte(out.String())

	var info HealInfo
	err = xml.Unmarshal(output, &info)
	if err != nil {

		logger.WithFields(log.Fields{
			"error": err.Error(),
		}).Error("Error unmarshalling from XML", err)

		return ""

	}

	JsonOutput, err := json.Marshal(&info.Bricks)
	if err != nil {

		logger.WithFields(log.Fields{
			"result": JsonOutput,
		}).Error("Error marshalling to JSON", err)

		return ""
	}

	return string(JsonOutput)
}

func glustershInfo(w http.ResponseWriter, r *http.Request) {
	p := mux.Vars(r)
	volname := p["name"]
	option := p["opts"]
	_ = option

	ctx := r.Context()

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

	JsonOutput := runGlfshealWithArgs(r, volname, option)

	restutils.SendHTTPResponse(ctx, w, http.StatusOK, JsonOutput)

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

	var option []string

	option = append(option, "granular-entry-heal-op")
	if err := txn.Ctx.Set("option", option); err != nil {
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
