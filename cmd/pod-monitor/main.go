package main

import (
	"context"
	"obs"
	"os"
	"os/signal"
	"syscall"
	"time"

	flags "github.com/jessevdk/go-flags"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"

	"k8s.io/client-go/rest"
)

type Options struct {
	obs.Options
}

func initOptions() *Options {
	var options Options
	parser := flags.NewParser(&options, flags.Default)

	if _, err := parser.Parse(); err != nil {
		os.Exit(1)
	}

	return &options
}

func main() {
	options := initOptions()
	rootCtx := context.Background()
	fr, closer := obs.InitGCP(rootCtx, "pod-monitor", options.LogLevel)
	defer closer()

	fs := fr.WithSpan(rootCtx)
	if err := run(rootCtx, fr, options); err != nil {
		fs.Critical("init", "exiting with error", obs.Vals{}.WithError(err))
		os.Exit(1)
	}

	fs.Info("clean shutdown", nil)
}

func run(ctx context.Context, fr obs.FlightRecorder, options *Options) error {
	fs := fr.WithSpan(ctx)
	config, err := rest.InClusterConfig()
	if err != nil {
		return err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)

	ticker := time.NewTicker(20 * time.Second)
	rpt := &reporter{lastRun: time.Now(), cs: clientset, fr: fr.ScopeName("pods")}
	for {
		select {
		case <-ticker.C:
			if err := rpt.report(ctx); err != nil {
				fs.Warn("report", "error reporting statuses", obs.Vals{}.WithError(err))
			}
		case <-sigs:
			return nil
		}
	}
}

type podKey struct {
	name      string
	namespace string
}

func (k podKey) fr(fr obs.FlightRecorder) obs.FlightRecorder {
	return fr.ScopeTags(obs.Tags{
		"pod_name":      k.name,
		"pod_namespace": k.namespace,
	})

}

type reporter struct {
	lastRun  time.Time
	lastPods map[podKey]struct{}
	cs       *kubernetes.Clientset
	fr       obs.FlightRecorder
}

func (r *reporter) report(ctx context.Context) error {
	pods, err := r.cs.Core().Pods("").List(v1.ListOptions{})
	if err != nil {
		return err
	}

	now := time.Now()
	secondsSinceLast := now.Sub(r.lastRun).Seconds()
	podSet := make(map[podKey]struct{})
	podsToDelete := r.lastPods
	for _, pod := range pods.Items {
		key := podKey{pod.ObjectMeta.Name, pod.ObjectMeta.Namespace}
		delete(podsToDelete, key)
		podSet[key] = struct{}{}

		podFR := key.fr(r.fr)

		podFS := podFR.WithSpan(ctx)
		podFS.IncrBy("phase."+string(pod.Status.Phase), float64(secondsSinceLast))
		podUp := float64(0)
		if pod.Status.Phase == v1.PodRunning {
			podUp = 1
		}
		podFS.SetGauge("running", podUp)

		for _, container := range pod.Status.ContainerStatuses {
			containerFR := podFR.ScopeTags(obs.Tags{
				"container_name": container.Name,
			})
			containerFS := containerFR.WithSpan(ctx)
			containerFS.SetGauge("container.restarts", float64(container.RestartCount))
		}
	}
	r.lastRun = now
	r.lastPods = podSet

	for k := range podsToDelete {
		podFR := k.fr(r.fr)
		podFS := podFR.WithSpan(ctx)
		podFS.SetGauge("running", 0)
	}
	return nil
}
