package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// Client wraps the Kubernetes client
type Client struct {
	clientset *kubernetes.Clientset
	namespace string
}

// NewClient creates a new Kubernetes client
// If namespace is empty, defaults to "ids"
func NewClient(namespace string) (*Client, error) {
	if namespace == "" {
		namespace = "ids"
	}

	config, err := getKubeConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return &Client{
		clientset: clientset,
		namespace: namespace,
	}, nil
}

// getKubeConfig gets the Kubernetes configuration
func getKubeConfig() (*rest.Config, error) {
	// Try in-cluster config first (when running inside Kubernetes)
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	// Fall back to kubeconfig file
	var kubeconfig string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	// Check if KUBECONFIG env var is set
	if envKubeconfig := os.Getenv("KUBECONFIG"); envKubeconfig != "" {
		kubeconfig = envKubeconfig
	}

	config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build config: %w", err)
	}

	return config, nil
}

// CreateEmailImportJob creates a Kubernetes Job for email import
func (c *Client) CreateEmailImportJob(ctx context.Context, jobName string, containerImage string) error {
	// Define the job
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: c.namespace,
			Labels: map[string]string{
				"app":          "email-import",
				"job-type":     "data-import",
				"triggered-by": "api",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            int32Ptr(3),
			TTLSecondsAfterFinished: int32Ptr(86400),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":      "email-import",
						"job-type": "data-import",
					},
				},
				Spec: c.buildPodSpec(containerImage),
			},
		},
	}

	// Create the job
	_, err := c.clientset.BatchV1().Jobs(c.namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}

	return nil
}

// buildPodSpec builds the pod spec for the email import job
func (c *Client) buildPodSpec(containerImage string) corev1.PodSpec {
	return corev1.PodSpec{
		RestartPolicy:      corev1.RestartPolicyNever,
		ServiceAccountName: "email-import-sa",
		InitContainers: []corev1.Container{
			{
				Name:  "download-emails",
				Image: "mcr.microsoft.com/azure-cli:latest",
				Command: []string{
					"/bin/sh",
					"-c",
					`set -e
echo ""
echo "==========================================="
echo "  EMAIL DOWNLOAD FROM AZURE BLOB STORAGE"
echo "==========================================="
echo "Storage: ${AZURE_STORAGE_ACCOUNT}"
echo "Container: ${AZURE_CONTAINER_NAME}"
echo "Started: $(date '+%Y-%m-%d %H:%M:%S')"
echo ""

mkdir -p /emails
START_TIME=$(date +%s)

# Check if files already exist (from previous runs)
EXISTING_SIZE=$(du -hs /emails 2>/dev/null | cut -f1 || echo "0")
FILE_COUNT=$(find /emails -type f 2>/dev/null | wc -l | tr -d ' ')

if [ "$FILE_COUNT" -gt 0 ]; then
  echo "===== Files Already Downloaded ====="
  echo "Found $FILE_COUNT existing files ($EXISTING_SIZE)"
  echo "Skipping download - using cached files from previous run"
  echo "===== Download Skipped ====="
else
  echo "No existing files found - downloading from Azure..."
  az storage blob download-batch \
    --account-name ${AZURE_STORAGE_ACCOUNT} \
    --account-key ${AZURE_STORAGE_KEY} \
    --source ${AZURE_CONTAINER_NAME} \
    --destination /emails \
    --pattern "*" \
    --output table
  
  END_TIME=$(date +%s)
  DURATION=$((END_TIME - START_TIME))
  MINUTES=$((DURATION / 60))
  SECONDS=$((DURATION % 60))
  
  echo ""
  echo "==========================================="
  echo "  DOWNLOAD COMPLETE"
  echo "==========================================="
  echo "Finished: $(date '+%Y-%m-%d %H:%M:%S')"
  echo "Duration: ${MINUTES}m ${SECONDS}s"
  
  FINAL_SIZE=$(du -hs /emails 2>/dev/null | cut -f1 || echo "unknown")
  FILE_COUNT=$(find /emails -type f 2>/dev/null | wc -l | tr -d ' ')
  
  echo "Total Size: $FINAL_SIZE"
  echo "File Count: $FILE_COUNT"
  echo "==========================================="
fi`,
				},
				Env: []corev1.EnvVar{
					{
						Name: "AZURE_STORAGE_ACCOUNT",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "azure-storage-secret",
								},
								Key: "storage-account-name",
							},
						},
					},
					{
						Name: "AZURE_STORAGE_KEY",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "azure-storage-secret",
								},
								Key: "storage-account-key",
							},
						},
					},
					{
						Name:  "AZURE_CONTAINER_NAME",
						Value: "email-imports",
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "email-data",
						MountPath: "/emails",
					},
				},
			},
		},
		Containers: []corev1.Container{
			{
				Name:  "import-emails",
				Image: containerImage,
				Command: []string{
					"/bin/sh",
					"-c",
					`set -e
echo "===== Starting Email Import Process ====="
eml_count=$(find /emails -name "*.eml" -type f | wc -l)
mbox_count=$(find /emails -name "*.mbox" -type f | wc -l)
echo "Found $eml_count EML files and $mbox_count MBOX files"
if [ "$eml_count" -gt 0 ]; then
  echo "===== Importing EML files ====="
  /home/appuser/import-emails -eml /emails -embeddings=true
fi
if [ "$mbox_count" -gt 0 ]; then
  echo "===== Importing MBOX files ====="
  find /emails -name "*.mbox" -type f | while read mbox_file; do
    echo "Processing: $mbox_file"
    /home/appuser/import-emails -mbox "$mbox_file" -embeddings=true
  done
fi
echo "===== Email Import Complete ====="`,
				},
				Env: []corev1.EnvVar{
					{
						Name: "DATABASE_URL",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "ids-secrets",
								},
								Key: "DATABASE_URL",
							},
						},
					},
					{
						Name: "OPENAI_API_KEY",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "ids-secrets",
								},
								Key: "OPENAI_API_KEY",
							},
						},
					},
					{
						Name:  "WAIT_FOR_TUNNEL",
						Value: "false",
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "email-data",
						MountPath: "/emails",
					},
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resourceQuantity("1Gi"),
						corev1.ResourceCPU:    resourceQuantity("500m"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: resourceQuantity("4Gi"),
						corev1.ResourceCPU:    resourceQuantity("2000m"),
					},
				},
			},
		},
		Volumes: []corev1.Volume{
			{
				Name: "email-data",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "email-data-pvc",
					},
				},
			},
		},
	}
}

// GetJobStatus gets the status of a job
func (c *Client) GetJobStatus(ctx context.Context, jobName string) (*batchv1.Job, error) {
	job, err := c.clientset.BatchV1().Jobs(c.namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}
	return job, nil
}

// DeleteJob deletes a job
func (c *Client) DeleteJob(ctx context.Context, jobName string) error {
	deletePolicy := metav1.DeletePropagationForeground
	err := c.clientset.BatchV1().Jobs(c.namespace).Delete(ctx, jobName, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	})
	if err != nil {
		return fmt.Errorf("failed to delete job: %w", err)
	}
	return nil
}

// Helper functions

func int32Ptr(i int32) *int32 {
	return &i
}

func resourceQuantity(value string) resource.Quantity {
	qty, err := resource.ParseQuantity(value)
	if err != nil {
		// Return zero quantity on error
		return resource.Quantity{}
	}
	return qty
}
