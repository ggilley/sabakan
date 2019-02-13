package etcd

import (
	"context"
	"errors"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/clientv3/clientv3util"
	"github.com/coreos/etcd/clientv3/concurrency"
	"github.com/cybozu-go/log"
	"github.com/cybozu-go/sabakan"
)

const noVersion = "1"

func (d *driver) Version(ctx context.Context) (string, error) {
RETRY:
	resp, err := d.client.Get(ctx, KeyVersion)
	if err != nil {
		return "", err
	}

	if resp.Count > 0 {
		return string(resp.Kvs[0].Value), nil
	}

	resp, err = d.client.Get(ctx, KeyIPAM)
	if err != nil {
		return "", err
	}
	if resp.Count > 0 {
		return noVersion, nil
	}

	// For sabakan < 1.2.0, when IPAM config is not set, convertTo2 does nothing.
	// Therefore it is safe to set schema version to sabakan.SchemaVersion as an
	// initialization.
	tresp, err := d.client.Txn(ctx).
		If(clientv3util.KeyMissing(KeyVersion)).
		Then(clientv3.OpPut(KeyVersion, sabakan.SchemaVersion)).
		Commit()
	if err != nil {
		return "", err
	}
	if !tresp.Succeeded {
		goto RETRY
	}
	return sabakan.SchemaVersion, nil
}

func (d *driver) Upgrade(ctx context.Context) error {
	sess, err := concurrency.NewSession(d.client)
	if err != nil {
		return err
	}
	defer sess.Close()

	mu := concurrency.NewMutex(sess, KeySchemaLockPrefix)
	if err := mu.Lock(ctx); err != nil {
		return err
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		mu.Unlock(ctx)
		cancel()
	}()

	sv, err := d.Version(ctx)
	if err != nil {
		return err
	}

	if sv == sabakan.SchemaVersion {
		return nil
	}

	log.Info("upgrading schema version", map[string]interface{}{
		"from": sv,
		"to":   sabakan.SchemaVersion,
	})

	switch sv {
	case "1":
		err := d.convertTo2(ctx, mu)
		if err != nil {
			return err
		}

		// fallthrough when case "2" is added
		//fallthrough
	default:
		return errors.New("unknown schema version: " + sv)
	}

	return nil
}
