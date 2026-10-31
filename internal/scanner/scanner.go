package scanner

import (
	"bytes"
	"encoding/json"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/yaml"

	v1alpha1 "github.com/Jamshaa/Umbra/api/v1alpha1"
)

// DetectorName is recorded on every Finding this package produces.
const DetectorName = "scanner"

// Heuristic thresholds for static detection. They are deliberately
// conservative: flagging a healthy manifest is worse than missing an
// edge case (the verifier and the sandbox decide the fix, not us).
var (
	// minSaneMemory: below this, a memory limit is a crashloop waiting
	// to happen (F02 injects 8Mi).
	minSaneMemory = resource.MustParse("16Mi")
	// maxSaneCPU / maxSaneMemory: above this, requests can never be
	// scheduled on a normal node (F03 injects cpu 64 / memory 1Ti).
	maxSaneCPU    = resource.MustParse("32")
	maxSaneMemory = resource.MustParse("512Gi")
)

// resourceRequirements mirrors the resources block of a container.
// Presence (not values) is what F01 cares about.
type resourceRequirements struct {
	Limits   map[string]string `json:"limits,omitempty"`
	Requests map[string]string `json:"requests,omitempty"`
}

// container is the subset of a container spec the detector reads.
type container struct {
	Name           string               `json:"name"`
	Image          string               `json:"image,omitempty"`
	Resources      resourceRequirements `json:"resources,omitempty"`
	LivenessProbe  *struct{}            `json:"livenessProbe,omitempty"`
	ReadinessProbe *struct{}            `json:"readinessProbe,omitempty"`
}

// podSpec is the subset of a pod template the detector reads.
type podSpec struct {
	Containers       []container `json:"containers"`
	ImagePullSecrets []struct {
		Name string `json:"name"`
	} `json:"imagePullSecrets,omitempty"`
}

// manifest is the subset of a workload object the detector reads.
type manifest struct {
	Kind     string `json:"kind"`
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		Template struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Spec podSpec `json:"spec"`
		} `json:"template"`
		// Inline pod spec (for Pod objects).
		Containers       []container `json:"containers"`
		ImagePullSecrets []struct {
			Name string `json:"name"`
		} `json:"imagePullSecrets,omitempty"`
	} `json:"spec"`
}

// podSpecOf returns the effective pod spec of a manifest: the template
// spec for workload objects, the inline spec for Pod objects.
func podSpecOf(m manifest) podSpec {
	if len(m.Spec.Template.Spec.Containers) > 0 || len(m.Spec.Template.Spec.ImagePullSecrets) > 0 {
		return m.Spec.Template.Spec
	}
	return podSpec{
		Containers:       m.Spec.Containers,
		ImagePullSecrets: m.Spec.ImagePullSecrets,
	}
}

// containersOf returns the containers of a Deployment-like or Pod manifest.
func containersOf(m manifest) []container {
	return podSpecOf(m).Containers
}

// decodeManifests splits multi-document YAML and decodes each document.
// Unknown kinds decode to zero values and are skipped by the detectors.
func decodeManifests(docs []byte) []manifest {
	var out []manifest
	for _, doc := range bytes.Split(docs, []byte("\n---")) {
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}
		js, err := yaml.ToJSON(doc)
		if err != nil {
			continue
		}
		var m manifest
		if err := json.Unmarshal(js, &m); err != nil {
			continue
		}
		out = append(out, m)
	}
	return out
}

// newFinding builds a scanner Finding for one container.
func newFinding(faultID string, category v1alpha1.FindingCategory, m manifest, c container) v1alpha1.Finding {
	ns := m.Metadata.Namespace
	name := m.Metadata.Name
	if name == "" {
		name = m.Spec.Template.Metadata.Name
	}
	return v1alpha1.Finding{
		Spec: v1alpha1.FindingSpec{
			Severity: v1alpha1.FindingSeverityMedium,
			Category: category,
			TargetRef: v1alpha1.TargetRef{
				Kind:      m.Kind,
				Namespace: ns,
				Name:      name,
			},
			Detector: DetectorName,
			State:    v1alpha1.FindingStateDetected,
		},
	}
}

// DetectNoResources finds containers with no resources block at all (F01).
func DetectNoResources(docs []byte) []v1alpha1.Finding {
	var out []v1alpha1.Finding
	for _, m := range decodeManifests(docs) {
		for _, c := range containersOf(m) {
			if len(c.Resources.Limits) == 0 && len(c.Resources.Requests) == 0 {
				out = append(out, newFinding("F01", v1alpha1.FindingCategoryResources, m, c))
			}
		}
	}
	return out
}

// DetectTinyMemoryLimit finds containers whose memory limit is below the
// sane minimum: an OOMKill loop waiting to happen (F02).
func DetectTinyMemoryLimit(docs []byte) []v1alpha1.Finding {
	var out []v1alpha1.Finding
	for _, m := range decodeManifests(docs) {
		for _, c := range containersOf(m) {
			raw, ok := c.Resources.Limits["memory"]
			if !ok {
				continue
			}
			q, err := resource.ParseQuantity(raw)
			if err != nil {
				continue
			}
			if q.Cmp(minSaneMemory) < 0 {
				out = append(out, newFinding("F02", v1alpha1.FindingCategoryResources, m, c))
			}
		}
	}
	return out
}

// DetectHugeRequests finds containers whose requests no normal node can
// satisfy: the pod stays Pending forever (F03).
func DetectHugeRequests(docs []byte) []v1alpha1.Finding {
	var out []v1alpha1.Finding
	for _, m := range decodeManifests(docs) {
		for _, c := range containersOf(m) {
			huge := false
			if raw, ok := c.Resources.Requests["cpu"]; ok {
				if q, err := resource.ParseQuantity(raw); err == nil && q.Cmp(maxSaneCPU) > 0 {
					huge = true
				}
			}
			if raw, ok := c.Resources.Requests["memory"]; ok {
				if q, err := resource.ParseQuantity(raw); err == nil && q.Cmp(maxSaneMemory) > 0 {
					huge = true
				}
			}
			if huge {
				out = append(out, newFinding("F03", v1alpha1.FindingCategoryResources, m, c))
			}
		}
	}
	return out
}

// DetectNoLivenessProbe finds containers with no liveness probe (F05).
func DetectNoLivenessProbe(docs []byte) []v1alpha1.Finding {
	var out []v1alpha1.Finding
	for _, m := range decodeManifests(docs) {
		for _, c := range containersOf(m) {
			if c.LivenessProbe == nil {
				out = append(out, newFinding("F05", v1alpha1.FindingCategoryReliability, m, c))
			}
		}
	}
	return out
}

// DetectNoReadinessProbe finds containers with no readiness probe (F06).
func DetectNoReadinessProbe(docs []byte) []v1alpha1.Finding {
	var out []v1alpha1.Finding
	for _, m := range decodeManifests(docs) {
		for _, c := range containersOf(m) {
			if c.ReadinessProbe == nil {
				out = append(out, newFinding("F06", v1alpha1.FindingCategoryReliability, m, c))
			}
		}
	}
	return out
}

// isPrivateRegistry reports whether image names a registry that needs
// credentials: an explicit host that is not Docker Hub, or localhost.
// Bare names ("nginx") and docker.io paths are public and need nothing.
func isPrivateRegistry(image string) bool {
	slash := -1
	for i := 0; i < len(image); i++ {
		if image[i] == '/' {
			slash = i
			break
		}
		if image[i] == ':' && slash < 0 {
			return false // tag on a bare name, no registry involved
		}
	}
	if slash < 0 {
		return false
	}
	host := image[:slash]
	if host == "localhost" {
		return true
	}
	dot := false
	for i := 0; i < len(host); i++ {
		if host[i] == '.' || host[i] == ':' {
			dot = true
			break
		}
	}
	if !dot {
		return false // docker.io user/repo form
	}
	return host != "docker.io" && host != "index.docker.io" && host != "registry-1.docker.io"
}

// DetectMissingImagePullSecrets finds containers pulling from a private
// registry with no imagePullSecrets on the pod (F21): the kubelet can
// never authenticate, so the pod never starts.
func DetectMissingImagePullSecrets(docs []byte) []v1alpha1.Finding {
	var out []v1alpha1.Finding
	for _, m := range decodeManifests(docs) {
		spec := podSpecOf(m)
		if len(spec.ImagePullSecrets) > 0 {
			continue
		}
		for _, c := range spec.Containers {
			if isPrivateRegistry(c.Image) {
				out = append(out, newFinding("F21", v1alpha1.FindingCategorySecurity, m, c))
			}
		}
	}
	return out
}

// Scan runs every detector over the manifests and returns all
// findings. Deterministic: same input, same output, no AI involved.
func Scan(docs []byte) []v1alpha1.Finding {
	var out []v1alpha1.Finding
	out = append(out, DetectNoResources(docs)...)
	out = append(out, DetectTinyMemoryLimit(docs)...)
	out = append(out, DetectHugeRequests(docs)...)
	out = append(out, DetectNoLivenessProbe(docs)...)
	out = append(out, DetectNoReadinessProbe(docs)...)
	out = append(out, DetectMissingImagePullSecrets(docs)...)
	return out
}
