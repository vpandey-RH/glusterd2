package api

// BrickReq represents Brick Request
type BrickReq struct {
	Type   string `json:"type"`
	NodeID string `json:"nodeid"`
	Path   string `json:"path"`
}

// SubvolReq represents Sub volume Request
type SubvolReq struct {
	Name         string      `json:"name"`
	Type         string      `json:"type"`
	Bricks       []BrickReq  `json:"bricks"`
	Subvols      []SubvolReq `json:"subvols"`
	ReplicaCount int         `json:"replica"`
	ArbiterCount int         `json:"arbiter"`
}

// VolCreateReq represents a Volume Create Request
type VolCreateReq struct {
	Name      string            `json:"name"`
	Transport string            `json:"transport,omitempty"`
	Subvols   []SubvolReq       `json:"subvols"`
	Options   map[string]string `json:"options,omitempty"`
	Force     bool              `json:"force,omitempty"`
}

// VolOptionReq represents an incoming request to set volume options
type VolOptionReq struct {
	Options map[string]string `json:"options"`
}

// VolExpandReq represents a request to expand the volume by adding more bricks
type VolExpandReq struct {
	ReplicaCount int        `json:"replica,omitempty"`
	Bricks       []BrickReq `json:"bricks"`
}
