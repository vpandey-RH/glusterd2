package glustershd

import (
        "fmt"

	"github.com/gluster/glusterd2/glusterd2/daemon"
	"github.com/gluster/glusterd2/glusterd2/transaction"
	"github.com/gluster/glusterd2/glusterd2/volume"
)


func selfhealdAction(c transaction.TxnCtx, action string) error{
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

func txnSelfHealStart(c transaction.TxnCtx) error {
        return selfhealdAction(c, "actionStart")
}

func txnSelfHealStop(c transaction.TxnCtx) error {
        return selfhealdAction(c, "actionStop")
}
