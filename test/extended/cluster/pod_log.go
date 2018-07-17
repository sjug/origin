package cluster

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclientset "k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("PODLOG", func() {
	defer g.GinkgoRecover()
	var (
		c               kclientset.Interface
		oc              = exutil.NewCLI("pod-log", exutil.KubeConfigPath())
		podName         = "pause-amd64"
		podLabel        = exutil.ParseLabelsOrDie(fmt.Sprintf("name=%s", podName))
		systemNamespace = "kube-system"
	)

	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "pause-amd64-",
			Labels: map[string]string{
				"name": podName,
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "write-pod",
					Image: "gcr.io/google_containers/pause-amd64:3.0",
					Ports: []v1.ContainerPort{
						{
							ContainerPort: 8080,
							Protocol:      v1.ProtocolTCP,
						},
					},
					ImagePullPolicy: v1.PullIfNotPresent,
				},
			},
			RestartPolicy: v1.RestartPolicyAlways,
		},
	}

	reLiteral := [11]string{
		`(?m)^I(\d\d)(\d+\s+\d+:\d+:\d+.\d+)\s+\d+\s+(factory.go:1147])\s(.*)$`,
		`(?m)^I(\d\d)(\d+\s+\d+:\d+:\d+.\d+)\s+\d+\s+(config.go:405])\s(.*)$`,
		`(?m)^I(\d\d)(\d+\s+\d+:\d+:\d+.\d+)\s+\d+\s+(kubelet_pods.go:1337])\s(.*)$`,
		`(?m)^I(\d\d)(\d+\s+\d+:\d+:\d+.\d+)\s+\d+\s+(desired_state_of_world_populator.go:302])\s(.*)$`,
		`(?m)^I(\d\d)(\d+\s+\d+:\d+:\d+.\d+)\s+\d+\s+(mount_linux.go:143])\s(.*)$`,
		`(?m)^I(\d\d)(\d+\s+\d+:\d+:\d+.\d+)\s+\d+\s+(kuberuntime_manager.go:385])\s(.*)$`,
		`(?m)^I(\d\d)(\d+\s+\d+:\d+:\d+.\d+)\s+\d+\s+(kuberuntime_manager.go:654])\s(.*)$`,
		`(?m)^I(\d\d)(\d+\s+\d+:\d+:\d+.\d+)\s+\d+\s+(kuberuntime_manager.go:724])\s(.*)$`,
		`(?m)^I(\d\d)(\d+\s+\d+:\d+:\d+.\d+)\s+\d+\s+(server.go:435])\s(.*)\s(Created\scontainer)$`,
		`(?m)^I(\d\d)(\d+\s+\d+:\d+:\d+.\d+)\s+\d+\s+(server.go:435])\s(.*)\s(Started\scontainer)$`,
		`(?m)^I(\d\d)(\d+\s+\d+:\d+:\d+.\d+)\s+\d+\s+(status_manager.go:146])\s(.*)\(2,\s{(Running)(.*)$`,
	}

	description := [10]string{
		"Schedule pod to node",
		"Node marks pod pending",
		"Time to start first volume mount",
		"Mount command occurs",
		"First mount completed",
		"Time to create podsandbox",
		"Time between podsandbox and container creating",
		"Container created",
		"Container started",
		"Pod running",
	}

	g.BeforeEach(func() {
		c = oc.AdminKubeClient()
	})

	g.It("Create a pod", func() {
		namespace := oc.Namespace()

		nodeList := e2e.GetReadySchedulableNodesOrDie(c)
		for i := range nodeList.Items {
			e2e.Logf("Node %d: %+v\n", i, nodeList.Items[i])
		}

		now := time.Now()

		podsCreated, err := c.CoreV1().Pods(namespace).Create(pod)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Pod created: %+v", podsCreated)

		_, err = exutil.WaitForPods(oc.KubeClient().Core().Pods(oc.Namespace()), podLabel, exutil.CheckPodIsRunning, 1, 4*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		controlPlane := []string{"controllers", "api"}
		var controllerName, apiName string
		for i := range controlPlane {
			podList, err := c.CoreV1().Pods(systemNamespace).List(metav1.ListOptions{LabelSelector: exutil.ParseLabelsOrDie(fmt.Sprintf("openshift.io/component=%s", controlPlane[i])).String()})
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("There were %d pods matching.", len(podList.Items))

			for j := range podList.Items {
				e2e.Logf("%s pod %d: %+v\n", controlPlane[i], j, podList.Items[j])
			}

			if controlPlane[i] == "controllers" {
				controllerName = podList.Items[0].Name
			} else if controlPlane[i] == "api" {
				apiName = podList.Items[0].Name
			}

		}
		e2e.Logf("The controller name is: %v", controllerName)
		e2e.Logf("The api name is: %v", apiName)

		var logBuffer bytes.Buffer

		controllerLog, err := oc.AsAdmin().Run("logs").Args(controllerName, "--namespace", systemNamespace, "--since-time", now.Format(time.RFC3339)).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("CONTROLLER LOG OUTPUT: %v", controllerLog)
		logBuffer.WriteString(controllerLog)

		apiLog, err := oc.AsAdmin().Run("logs").Args(apiName, "--namespace", systemNamespace, "--since-time", now.Format(time.RFC3339)).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("API LOG OUTPUT: %v", apiLog)
		logBuffer.WriteString(apiLog)

		journalTimeFormat := fmt.Sprintf("%d-%02d-%02d %02d:%02d:%02d", now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second())
		kubeletLog, err := exec.Command("/usr/bin/ssh", fmt.Sprintf("root@%s", nodeList.Items[0].Name), fmt.Sprintf("journalctl -u atomic-openshift-node --since \"%s\" -o cat", journalTimeFormat)).Output()
		if err != nil {
			fmt.Printf("Error: %v", err)
		}
		e2e.Logf("KUBELET LOG OUTPUT: %v", string(kubeletLog))
		logBuffer.WriteString(string(kubeletLog))

		log := logBuffer.String()

		results := [][]string{}
		for i := range reLiteral {
			re := regexp.MustCompile(reLiteral[i])
			result := re.FindStringSubmatch(log)
			results = append(results, result)
		}

		timestamps := []time.Time{}
		for i, result := range results {
			if len(result) > 0 {
				t2, err := goodTime(result)
				if err != nil {
					fmt.Printf("Error: %v", err)
				}
				timestamps = append(timestamps, t2)
			} else {
				e2e.Logf("Result %d: NO REGEX FOUND\n", i)
			}
		}

		for i := 0; i < len(timestamps)-1; i++ {
			diff := timestamps[i+1].Sub(timestamps[i])
			fmt.Printf("T%d, %s: %v\n", i, description[i], diff)
		}
	})
})

func goodTime(result []string) (time.Time, error) {
	if len(result) < 3 {
		return time.Time{}, errors.New("Result list invalid index")
	}

	i, err := strconv.Atoi(result[1])
	if err != nil {
	}
	month := fmt.Sprintf("%v", time.Month(i))
	timestamp := month[:3] + " " + result[2]

	t, err := time.Parse(time.StampMicro, timestamp)
	if err != nil {
		return time.Time{}, err
	}

	return t, nil
}
