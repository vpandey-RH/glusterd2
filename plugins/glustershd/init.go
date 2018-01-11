package glustershd

import (
	"github.com/gluster/glusterd2/glusterd2/servers/rest/route"
	"github.com/gluster/glusterd2/glusterd2/transaction"
	"github.com/prashanthpai/sunrpc"
)

// Plugin is a structure which implements GlusterdPlugin interface
type Plugin struct {
}

// Name returns name of plugin
func (p *Plugin) Name() string {
	return "glustershd"
}

// SunRPCProgram returns sunrpc program to register with Glusterd
func (p *Plugin) SunRPCProgram() sunrpc.Program {
	return nil
}

// RestRoutes returns list of REST API routes to register with Glusterd
func (p *Plugin) RestRoutes() route.Routes {
	return route.Routes{
		route.Route{
			Name:        "GlustershEnable",
			Method:      "POST",
			Pattern:     "/volumes/{name}/heal/enable",
			Version:     1,
			HandlerFunc: glustershEnableHandler},
		route.Route{
			Name:        "GlustershDisable",
			Method:      "POST",
			Pattern:     "/volumes/{name}/heal/disable",
			Version:     1,
			HandlerFunc: glustershDisableHandler},
		route.Route{
			Name:        "GlustershInfo",
			Method:      "GET",
			Pattern:     "/volumes/{name}/{opts}/heal-info",
			Version:     1,
			HandlerFunc: glustershInfo},
		route.Route{
			Name:        "GlustershInfo2",
			Method:      "GET",
			Pattern:     "/volumes/{name}/heal-info",
			Version:     1,
			HandlerFunc: glustershInfo},
		route.Route{
			Name:        "GranularHealEnable",
			Method:      "POST",
			Pattern:     "/volumes/{name}/heal/granular-entry/enable",
			Version:     1,
			HandlerFunc: granularHealEnableHandler},
	}
}

// RegisterStepFuncs registers transaction step functions with
// Glusterd Transaction framework
func (p *Plugin) RegisterStepFuncs() {
	transaction.RegisterStepFunc(txnSelfHealStart, "selfheal-start.Commit")
	transaction.RegisterStepFunc(txnSelfHealStop, "selfheal-stop.Commit")
	transaction.RegisterStepFunc(txnGranularEntryHealEnable, "granular-heal.Enable")
}
