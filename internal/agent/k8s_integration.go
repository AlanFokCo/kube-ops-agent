package agent

import (
	"os"

	"github.com/alanfokco/agentscope-go/v2/pkg/agentscope/tool"
	"github.com/alanfokco/agentscope-go/v2/pkg/agentscope/workspace"
)

// NewK8sClusterTools creates read-only K8s cluster query tools powered by
// agentscope-go v2. These provide structured, safe access for LLM
// self-planning: resource listing and log retrieval.
// Secrets are explicitly blocked by the framework.
func NewK8sClusterTools() []tool.Tool {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, _ := os.UserHomeDir()
		if home != "" {
			kubeconfig = home + "/.kube/config"
		}
	}

	return []tool.Tool{
		workspace.NewKubectlGetTool(kubeconfig),
		workspace.NewKubectlLogTool(kubeconfig),
	}
}
