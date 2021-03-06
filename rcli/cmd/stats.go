package cmd

import (
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/mhausenblas/reshifter/pkg/discovery"
	"github.com/mhausenblas/reshifter/pkg/types"
	"github.com/spf13/cobra"
)

// statsCmd represents the stats command
var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Collects stats about Kubernetes-related keys from an etcd endpoint",
	Run: func(cmd *cobra.Command, args []string) {
		ep := cmd.Flag("endpoint").Value.String()
		fmt.Printf("Collecting stats from etcd endpoint %s\n", ep)
		docollectstats(ep)
	},
}

func init() {
	RootCmd.AddCommand(statsCmd)
	statsCmd.Flags().StringP("endpoint", "e", "http://127.0.0.1:2379", "The URL of the etcd to collect stats from")
}

func docollectstats(endpoint string) {
	if endpoint == "" || strings.Index(endpoint, "http") != 0 {
		merr := "The endpoint is malformed"
		log.Error(merr)
		return
	}
	_, _, err := discovery.ProbeEtcd(endpoint)
	if err != nil {
		log.Errorf(fmt.Sprintf("%s", err))
		return
	}
	vk, vs, err := discovery.CountKeysFor(endpoint, types.Vanilla)
	if err != nil {
		log.Error(fmt.Sprintf("Having problems calculating stats: %s", err))
		return
	}
	fmt.Printf("Vanilla Kubernetes [keys:%d, size:%d]\n", vk, vs)
	osk, oss, _ := discovery.CountKeysFor(endpoint, types.OpenShift)
	if osk > 0 {
		fmt.Printf("OpenShift [keys:%d, size:%d]\n\n", osk, oss)
	}
}
