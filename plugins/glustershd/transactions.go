package glustershd

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/gluster/glusterd2/glusterd2/daemon"
	"github.com/gluster/glusterd2/glusterd2/gdctx"
	"github.com/gluster/glusterd2/glusterd2/transaction"
	"github.com/gluster/glusterd2/glusterd2/volume"
)

func selfhealdAction(c transaction.TxnCtx, action string) error {
	glustershDaemon, err := newGlustershd()
	if err != nil {
		return err
	}
	switch action {
	case "actionStart":
		err = daemon.Start(glustershDaemon, true)
	case "actionStop":
		if volume.CheckReplicateVolumesStatus() {
			err = daemon.Stop(glustershDaemon, true)
		} else {
			fmt.Println("volumes started")
		}
	}

	return err
}

func runGlfshealBin(c transaction.TxnCtx, volname string, option []string) error {
	var out bytes.Buffer
	var buffer bytes.Buffer

	buffer.WriteString(fmt.Sprintf("%s", volname))
	for _, opt := range option {
		buffer.WriteString(fmt.Sprintf(" %s", opt))
	}

	a := buffer.String()
	args := strings.Fields(a)
	path, e := exec.LookPath("glfsheal")

	if e != nil {

		fmt.Println("Couldn't Find glfsheal binary")

		return e
	}

	cmd := exec.Command(path, args...)
	fmt.Println(cmd)
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {

		fmt.Println("Failed Running Glfsheal Binary")

		return err
	}

	c.SetNodeResult(gdctx.MyUUID, "stdout", out.String())

	return nil
}

func txnSelfHealStart(c transaction.TxnCtx) error {
	return selfhealdAction(c, "actionStart")
}

func txnSelfHealStop(c transaction.TxnCtx) error {
	return selfhealdAction(c, "actionStop")
}

func txnHealInfo(c transaction.TxnCtx) error {
	var volname string
	var option []string

	if err := c.Get("volname", &volname); err != nil {
		return err
	}

	if err := c.Get("option", &option); err != nil {
		return err
	}

	return runGlfshealBin(c, volname, option)

}

func txnGranularEntryHealEnable(c transaction.TxnCtx) error {
	var volname string
	var option []string

	if err := c.Get("volname", &volname); err != nil {
		return err
	}

	if err := c.Get("option", &option); err != nil {
		return err
	}

	return runGlfshealBin(c, volname, option)
}
