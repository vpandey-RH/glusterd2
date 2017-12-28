package volume

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/coreos/etcd/clientv3"
	"github.com/gluster/glusterd2/glusterd2/store"
	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
)

const (
	volumePrefix string = "volumes/"
)

var (
	//ExistsFunc check whether a given volume exist or not
	ExistsFunc = Exists
	// AddOrUpdateVolumeFunc marshals to volume object and passes to store to add/update
	AddOrUpdateVolumeFunc = AddOrUpdateVolume
)

// AddOrUpdateVolume marshals to volume object and passes to store to add/update
func AddOrUpdateVolume(v *Volinfo) error {
	json, e := json.Marshal(v)
	if e != nil {
		log.WithField("error", e).Error("Failed to marshal the volinfo object")
		return e
	}

	_, e = store.Store.Put(context.TODO(), volumePrefix+v.Name, string(json))
	if e != nil {
		log.WithError(e).Error("Couldn't add volume to store")
		return e
	}
	return nil
}

// GetVolume fetches the json object from the store and unmarshalls it into
// volinfo object
func GetVolume(name string) (*Volinfo, error) {
	var v Volinfo
	resp, e := store.Store.Get(context.TODO(), volumePrefix+name)
	if e != nil {
		log.WithError(e).Error("Couldn't retrive volume from store")
		return nil, e
	}

	if resp.Count != 1 {
		log.WithField("volume", name).Error("volume not found")
		return nil, errors.New("volume not found")
	}

	if e = json.Unmarshal(resp.Kvs[0].Value, &v); e != nil {
		log.WithError(e).Error("Failed to unmarshal the data into volinfo object")
		return nil, e
	}
	return &v, nil
}

//DeleteVolume passes the volname to store to delete the volume object
func DeleteVolume(name string) error {
	_, e := store.Store.Delete(context.TODO(), volumePrefix+name)
	return e
}

// GetVolumesList returns a map of volume names to their UUIDs
func GetVolumesList() (map[string]uuid.UUID, error) {
	resp, e := store.Store.Get(context.TODO(), volumePrefix, clientv3.WithPrefix())
	if e != nil {
		return nil, e
	}

	volumes := make(map[string]uuid.UUID)

	for _, kv := range resp.Kvs {
		var vol Volinfo

		if err := json.Unmarshal(kv.Value, &vol); err != nil {
			log.WithFields(log.Fields{
				"volume": string(kv.Key),
				"error":  err,
			}).Error("Failed to unmarshal volume")
			continue
		}

		volumes[vol.Name] = vol.ID
	}

	return volumes, nil
}

//GetVolumes retrives the json objects from the store and converts them into
//respective volinfo objects
func GetVolumes() ([]*Volinfo, error) {
	resp, e := store.Store.Get(context.TODO(), volumePrefix, clientv3.WithPrefix())
	if e != nil {
		return nil, e
	}

	volumes := make([]*Volinfo, len(resp.Kvs))

	for i, kv := range resp.Kvs {
		var vol Volinfo

		if err := json.Unmarshal(kv.Value, &vol); err != nil {
			log.WithFields(log.Fields{
				"volume": string(kv.Key),
				"error":  err,
			}).Error("Failed to unmarshal volume")
			continue
		}

		volumes[i] = &vol
	}

	return volumes, nil
}

// CheckReplicateVolumesStatus retrieves the voliinfo objects from GetVolumes() function
// and checks if all replicate, Disperse volumes are stopped before
// stopping the self heal daemon.
func CheckReplicateVolumesStatus() bool {
	volumes, e := GetVolumes()
	log.WithFields(log.Fields{
		"volume": volumes,
		"error":  e,
	}).Error("volume list")

	if e != nil {
		return false
	}
	for _, v := range volumes {
                log.WithFields(log.Fields{
		         "msg": v.Type,
		 }).Error("vol started")

		if v.Type != 1 && v.Type != 2 {
			continue
		} else {

			if v.State == 0 {
				continue
			} else {
                        	log.WithFields(log.Fields{
				        "msg": "vol started",
			        }).Error("vol started")

				return false
			}
		}
	}
	return true
}

//Exists check whether a given volume exist or not
func Exists(name string) bool {
	resp, e := store.Store.Get(context.TODO(), volumePrefix+name)
	if e != nil {
		return false
	}

	return resp.Count == 1
}
