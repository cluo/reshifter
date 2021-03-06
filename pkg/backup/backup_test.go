package backup

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/coreos/etcd/client"
	"github.com/mhausenblas/reshifter/pkg/types"
	"github.com/mhausenblas/reshifter/pkg/util"
	"github.com/prometheus/common/log"
)

var (
	tmpTestDir = "test/"
	storetests = []struct {
		path string
		val  string
	}{
		{"", ""},
		{"non-valid-key", ""},
		{"/", "root"},
		{"/" + tmpTestDir, "some"},
		{"/" + tmpTestDir + "/first-level", "another"},
		{"/" + tmpTestDir + "/this:also", "escaped"},
	}
)

func TestStore(t *testing.T) {
	for _, tt := range storetests {
		p, err := store(".", tt.path, tt.val)
		if err != nil {
			continue
		}
		c, _ := ioutil.ReadFile(p)
		got := string(c)
		if tt.path == "/" {
			_ = os.Remove(p)
		}
		want := tt.val
		if got != want {
			t.Errorf("backup.store(\".\", %q, %q) => %q, want %q", tt.path, tt.val, got, want)
		}
	}
	// make sure to clean up remaining directories:
	_ = os.RemoveAll(tmpTestDir)
}

func TestBackup(t *testing.T) {
	port := "4001"
	// testing insecure etcd 2 and 3:
	tetcd := "http://127.0.0.1:" + port
	// backing up to remote https://play.minio.io:9000:
	_ = os.Setenv("ACCESS_KEY_ID", "Q3AM3UQ867SPQQA43P2F")
	_ = os.Setenv("SECRET_ACCESS_KEY", "zuf+tfteSlswRu7BJ86wekitnifILbZam1KYY3TG")
	etcd2Backup(t, port, tetcd, types.Vanilla)
	etcd3Backup(t, port, tetcd, types.Vanilla)
	etcd2Backup(t, port, tetcd, types.OpenShift)
	etcd3Backup(t, port, tetcd, types.OpenShift)

	// testing secure etcd 2 and 3:
	tetcd = "https://127.0.0.1:" + port
	etcd2Backup(t, port, tetcd, types.Vanilla)
	etcd3Backup(t, port, tetcd, types.Vanilla)
	etcd2Backup(t, port, tetcd, types.OpenShift)
	etcd3Backup(t, port, tetcd, types.OpenShift)
}

func etcd2Backup(t *testing.T, port, tetcd string, distro types.KubernetesDistro) {
	defer func() { _ = util.EtcdDown() }()
	_ = os.Setenv("RS_ETCD_CLIENT_CERT", filepath.Join(util.Certsdir(), "client.pem"))
	_ = os.Setenv("RS_ETCD_CLIENT_KEY", filepath.Join(util.Certsdir(), "client-key.pem"))
	_ = os.Setenv("RS_ETCD_CA_CERT", filepath.Join(util.Certsdir(), "ca.pem"))
	secure, err := util.LaunchEtcd2(tetcd, port)
	if err != nil {
		t.Errorf("%s", err)
		return
	}
	c2, err := util.NewClient2(tetcd, secure)
	if err != nil {
		t.Errorf("Can't connect to local etcd2 at %s: %s", tetcd, err)
		return
	}
	kapi := client.NewKeysAPI(c2)
	testkey, testval, err := genentry("2", types.Vanilla)
	if err != nil {
		t.Errorf("%s", err)
		return
	}
	log.Infof("K:%s V:%s", testkey, testval)
	_, err = kapi.Set(context.Background(), testkey, testval, &client.SetOptions{Dir: false, PrevExist: client.PrevNoExist})
	if err != nil {
		t.Errorf("Can't create etcd entry %s=%s: %s", testkey, testval, err)
		return
	}
	if distro == types.OpenShift {
		testkey, testval, erro := genentry("2", types.OpenShift)
		if erro != nil {
			t.Errorf("%s", erro)
			return
		}
		log.Infof("K:%s V:%s", testkey, testval)
		_, err = kapi.Set(context.Background(), testkey, testval, &client.SetOptions{Dir: false, PrevExist: client.PrevNoExist})
		if err != nil {
			t.Errorf("Can't create etcd entry %s=%s: %s", testkey, testval, err)
			return
		}
	}
	backupid, err := Backup(tetcd, types.DefaultWorkDir, "play.minio.io:9000", "reshifter-test-cluster")
	if err != nil {
		t.Errorf("Error during backup: %s", err)
		return
	}
	opath, _ := filepath.Abs(filepath.Join(types.DefaultWorkDir, backupid))
	_, err = os.Stat(opath + ".zip")
	if err != nil {
		t.Errorf("No archive found: %s", err)
	}
	// make sure to clean up:
	_ = os.Remove(opath + ".zip")
}

func etcd3Backup(t *testing.T, port, tetcd string, distro types.KubernetesDistro) {
	defer func() { _ = util.EtcdDown() }()
	_ = os.Setenv("ETCDCTL_API", "3")
	_ = os.Setenv("RS_ETCD_CLIENT_CERT", filepath.Join(util.Certsdir(), "client.pem"))
	_ = os.Setenv("RS_ETCD_CLIENT_KEY", filepath.Join(util.Certsdir(), "client-key.pem"))
	_ = os.Setenv("RS_ETCD_CA_CERT", filepath.Join(util.Certsdir(), "ca.pem"))
	secure, err := util.LaunchEtcd3(tetcd, port)
	if err != nil {
		t.Errorf("%s", err)
		return
	}
	c3, err := util.NewClient3(tetcd, secure)
	if err != nil {
		t.Errorf("Can't connect to local etcd3 at %s: %s", tetcd, err)
		return
	}
	testkey, testval, err := genentry("3", distro)
	if err != nil {
		t.Errorf("%s", err)
		return
	}
	_, err = c3.Put(context.Background(), testkey, testval)
	if err != nil {
		t.Errorf("Can't create etcd entry %s=%s: %s", testkey, testval, err)
		return
	}
	// val, err := c3.Get(context.Background(), testkey)
	// if err != nil {
	// 	t.Errorf("Can't get etcd key %s: %s", testkey, err)
	// 	return
	// }
	backupid, err := Backup(tetcd, types.DefaultWorkDir, "play.minio.io:9000", "reshifter-test-cluster")
	if err != nil {
		t.Errorf("Error during backup: %s", err)
		return
	}
	opath, _ := filepath.Abs(filepath.Join(types.DefaultWorkDir, backupid))
	_, err = os.Stat(opath + ".zip")
	if err != nil {
		t.Errorf("No archive found: %s", err)
	}
	// make sure to clean up:
	_ = os.Remove(opath + ".zip")
}

func genentry(etcdversion string, distro types.KubernetesDistro) (string, string, error) {
	switch distro {
	case types.Vanilla:
		if etcdversion == "2" {
			return types.LegacyKubernetesPrefix + "/namespaces/kube-system", "{\"kind\":\"Namespace\",\"apiVersion\":\"v1\"}", nil
		}
		return types.KubernetesPrefix + "/namespaces/kube-system", "{\"kind\":\"Namespace\",\"apiVersion\":\"v1\"}", nil
	case types.OpenShift:
		return types.OpenShiftPrefix + "/builds", "{\"kind\":\"Build\",\"apiVersion\":\"v1\"}", nil
	default:
		return "", "", fmt.Errorf("That's not a Kubernetes distro")
	}
}
