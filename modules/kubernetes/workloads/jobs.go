package workloads

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/izetmolla/containerws/modules/kubernetes/kubecli"
)

type jobRow struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	Completions string `json:"completions"`
	Succeeded   int32  `json:"succeeded"`
	Failed      int32  `json:"failed"`
	Active      int32  `json:"active"`
	Duration    string `json:"duration,omitempty"`
	CreatedAt   string `json:"created_at"`
	Images      string `json:"images"`
}

func jobCompletions(j *batchv1.Job) string {
	succeeded := j.Status.Succeeded
	desired := int32(1)
	if j.Spec.Completions != nil {
		desired = *j.Spec.Completions
	}
	return fmt.Sprintf("%d/%d", succeeded, desired)
}

func jobDuration(j *batchv1.Job) string {
	if j.Status.StartTime == nil {
		return ""
	}
	end := time.Now()
	if j.Status.CompletionTime != nil {
		end = j.Status.CompletionTime.Time
	}
	d := end.Sub(j.Status.StartTime.Time).Round(time.Second)
	return d.String()
}

func (cc *controller) ListJobsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ns := kubecli.NamespaceQuery(c)
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	list, err := cli.BatchV1().Jobs(ns).List(ctx, metav1.ListOptions{Limit: 5000})
	if err != nil {
		return cc.respondErr(c, err)
	}
	rows := make([]jobRow, 0, len(list.Items))
	for i := range list.Items {
		j := &list.Items[i]
		var images []string
		for _, ctr := range j.Spec.Template.Spec.Containers {
			images = append(images, ctr.Image)
		}
		rows = append(rows, jobRow{
			Namespace:   j.Namespace,
			Name:        j.Name,
			Completions: jobCompletions(j),
			Succeeded:   j.Status.Succeeded,
			Failed:      j.Status.Failed,
			Active:      j.Status.Active,
			Duration:    jobDuration(j),
			CreatedAt:   j.CreationTimestamp.UTC().Format(time.RFC3339),
			Images:      strings.Join(images, ", "),
		})
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": rows}))
}

type createJobBody struct {
	Namespace     string   `json:"namespace"`
	Name          string   `json:"name"`
	Image         string   `json:"image"`
	Command       []string `json:"command"`
	Args          []string `json:"args"`
	Completions   *int32   `json:"completions"`
	Parallelism   *int32   `json:"parallelism"`
	BackoffLimit  *int32   `json:"backoff_limit"`
	RestartPolicy string   `json:"restart_policy"`
}

func (cc *controller) CreateJobAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var body createJobBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	ns := strings.TrimSpace(body.Namespace)
	name := strings.TrimSpace(body.Name)
	image := strings.TrimSpace(body.Image)
	if ns == "" {
		ns = "default"
	}
	if name == "" {
		return cc.respondErr(c, fmt.Errorf("name is required"))
	}
	if image == "" {
		return cc.respondErr(c, fmt.Errorf("image is required"))
	}

	restart := corev1.RestartPolicyNever
	switch strings.TrimSpace(body.RestartPolicy) {
	case string(corev1.RestartPolicyOnFailure):
		restart = corev1.RestartPolicyOnFailure
	case string(corev1.RestartPolicyNever), "":
		restart = corev1.RestartPolicyNever
	default:
		return cc.respondErr(c, fmt.Errorf("restart_policy must be Never or OnFailure"))
	}

	cmd := make([]string, 0, len(body.Command))
	for _, part := range body.Command {
		part = strings.TrimSpace(part)
		if part != "" {
			cmd = append(cmd, part)
		}
	}
	args := make([]string, 0, len(body.Args))
	for _, part := range body.Args {
		part = strings.TrimSpace(part)
		if part != "" {
			args = append(args, part)
		}
	}

	ctr := corev1.Container{
		Name:  "job",
		Image: image,
	}
	if len(cmd) > 0 {
		ctr.Command = cmd
	}
	if len(args) > 0 {
		ctr.Args = args
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: batchv1.JobSpec{
			Completions:  body.Completions,
			Parallelism:  body.Parallelism,
			BackoffLimit: body.BackoffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: restart,
					Containers:    []corev1.Container{ctr},
				},
			},
		},
	}

	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	created, err := cli.BatchV1().Jobs(ns).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": jobRow{
			Namespace:   created.Namespace,
			Name:        created.Name,
			Completions: jobCompletions(created),
			Succeeded:   created.Status.Succeeded,
			Failed:      created.Status.Failed,
			Active:      created.Status.Active,
			Duration:    jobDuration(created),
			CreatedAt:   created.CreationTimestamp.UTC().Format(time.RFC3339),
			Images:      image,
		},
		"message": "Job created",
	}))
}

func (cc *controller) GetJobAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()
	j, err := cli.BatchV1().Jobs(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	images := make([]string, 0)
	ctrs := make([]fiber.Map, 0)
	for _, ctr := range j.Spec.Template.Spec.Containers {
		images = append(images, ctr.Image)
		ctrs = append(ctrs, fiber.Map{"name": ctr.Name, "image": ctr.Image})
	}
	conds := make([]fiber.Map, 0, len(j.Status.Conditions))
	for _, cnd := range j.Status.Conditions {
		conds = append(conds, fiber.Map{
			"type":    string(cnd.Type),
			"status":  string(cnd.Status),
			"reason":  cnd.Reason,
			"message": cnd.Message,
		})
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"namespace":     j.Namespace,
			"name":          j.Name,
			"completions":   jobCompletions(j),
			"succeeded":     j.Status.Succeeded,
			"failed":        j.Status.Failed,
			"active":        j.Status.Active,
			"duration":      jobDuration(j),
			"parallelism":   j.Spec.Parallelism,
			"backoff_limit": j.Spec.BackoffLimit,
			"images":        images,
			"containers":    ctrs,
			"conditions":    conds,
			"labels":        j.Labels,
			"annotations":   j.Annotations,
			"created_at":    j.CreationTimestamp.UTC().Format(time.RFC3339),
		},
	}))
}

func (cc *controller) JobYAMLAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()
	j, err := cli.BatchV1().Jobs(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	y, err := kubecli.ToYAML(j)
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{"namespace": ns, "name": name, "yaml": y},
	}))
}

func (cc *controller) ApplyJobYAMLAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	var body applyYAMLBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	res, err := kubecli.ApplyYAML(cc.app, body.YAML, kubecli.ApplyExpect{
		APIVersion: "batch/v1",
		Kind:       "Job",
		Namespace:  ns,
		Name:       name,
	})
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    res,
		"message": "Job YAML applied",
	}))
}

func (cc *controller) DeleteJobAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()
	propagation := metav1.DeletePropagationBackground
	if err := cli.BatchV1().Jobs(ns).Delete(ctx, name, metav1.DeleteOptions{
		PropagationPolicy: &propagation,
	}); err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    fiber.Map{"namespace": ns, "name": name},
		"message": "Job deleted",
	}))
}

type cronRow struct {
	Namespace    string `json:"namespace"`
	Name         string `json:"name"`
	Schedule     string `json:"schedule"`
	Suspend      bool   `json:"suspend"`
	Active       int    `json:"active"`
	LastSchedule string `json:"last_schedule,omitempty"`
	CreatedAt    string `json:"created_at"`
	Images       string `json:"images"`
}

func (cc *controller) ListCronJobsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ns := kubecli.NamespaceQuery(c)
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	list, err := cli.BatchV1().CronJobs(ns).List(ctx, metav1.ListOptions{Limit: 5000})
	if err != nil {
		return cc.respondErr(c, err)
	}
	rows := make([]cronRow, 0, len(list.Items))
	for _, cj := range list.Items {
		var images []string
		for _, ctr := range cj.Spec.JobTemplate.Spec.Template.Spec.Containers {
			images = append(images, ctr.Image)
		}
		last := ""
		if cj.Status.LastScheduleTime != nil {
			last = cj.Status.LastScheduleTime.UTC().Format(time.RFC3339)
		}
		suspend := false
		if cj.Spec.Suspend != nil {
			suspend = *cj.Spec.Suspend
		}
		rows = append(rows, cronRow{
			Namespace:    cj.Namespace,
			Name:         cj.Name,
			Schedule:     cj.Spec.Schedule,
			Suspend:      suspend,
			Active:       len(cj.Status.Active),
			LastSchedule: last,
			CreatedAt:    cj.CreationTimestamp.UTC().Format(time.RFC3339),
			Images:       strings.Join(images, ", "),
		})
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": rows}))
}

func (cc *controller) GetCronJobAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()
	cj, err := cli.BatchV1().CronJobs(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	images := make([]string, 0)
	for _, ctr := range cj.Spec.JobTemplate.Spec.Template.Spec.Containers {
		images = append(images, ctr.Image)
	}
	suspend := false
	if cj.Spec.Suspend != nil {
		suspend = *cj.Spec.Suspend
	}
	last := ""
	if cj.Status.LastScheduleTime != nil {
		last = cj.Status.LastScheduleTime.UTC().Format(time.RFC3339)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"namespace":     cj.Namespace,
			"name":          cj.Name,
			"schedule":      cj.Spec.Schedule,
			"suspend":       suspend,
			"concurrency":   string(cj.Spec.ConcurrencyPolicy),
			"active":        len(cj.Status.Active),
			"last_schedule": last,
			"images":        images,
			"labels":        cj.Labels,
			"annotations":   cj.Annotations,
			"created_at":    cj.CreationTimestamp.UTC().Format(time.RFC3339),
		},
	}))
}

func (cc *controller) CronJobYAMLAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()
	cj, err := cli.BatchV1().CronJobs(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	y, err := kubecli.ToYAML(cj)
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{"namespace": ns, "name": name, "yaml": y},
	}))
}

func (cc *controller) ApplyCronJobYAMLAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	var body applyYAMLBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	res, err := kubecli.ApplyYAML(cc.app, body.YAML, kubecli.ApplyExpect{
		APIVersion: "batch/v1",
		Kind:       "CronJob",
		Namespace:  ns,
		Name:       name,
	})
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    res,
		"message": "CronJob YAML applied",
	}))
}

func (cc *controller) DeleteCronJobAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()
	if err := cli.BatchV1().CronJobs(ns).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    fiber.Map{"namespace": ns, "name": name},
		"message": "CronJob deleted",
	}))
}

type createCronJobBody struct {
	Namespace string            `json:"namespace"`
	Name      string            `json:"name"`
	Schedule  string            `json:"schedule"`
	Image     string            `json:"image"`
	Labels    map[string]string `json:"labels"`
	Command   []string          `json:"command"`
	Args      []string          `json:"args"`
	Suspend   bool              `json:"suspend"`
}

func (cc *controller) CreateCronJobAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var body createCronJobBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	ns := strings.TrimSpace(body.Namespace)
	name := strings.TrimSpace(body.Name)
	schedule := strings.TrimSpace(body.Schedule)
	image := strings.TrimSpace(body.Image)
	if ns == "" {
		ns = "default"
	}
	if name == "" {
		return cc.respondErr(c, fmt.Errorf("name is required"))
	}
	if schedule == "" {
		schedule = "0 * * * *"
	}
	if image == "" {
		return cc.respondErr(c, fmt.Errorf("image is required"))
	}

	matchLabels := map[string]string{}
	for k, v := range body.Labels {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		matchLabels[k] = strings.TrimSpace(v)
	}
	if len(matchLabels) == 0 {
		matchLabels["app"] = name
	}

	cmd := make([]string, 0, len(body.Command))
	for _, part := range body.Command {
		part = strings.TrimSpace(part)
		if part != "" {
			cmd = append(cmd, part)
		}
	}
	args := make([]string, 0, len(body.Args))
	for _, part := range body.Args {
		part = strings.TrimSpace(part)
		if part != "" {
			args = append(args, part)
		}
	}
	ctr := corev1.Container{Name: "main", Image: image}
	if len(cmd) > 0 {
		ctr.Command = cmd
	}
	if len(args) > 0 {
		ctr.Args = args
	}
	suspend := body.Suspend
	cj := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    matchLabels,
		},
		Spec: batchv1.CronJobSpec{
			Schedule: schedule,
			Suspend:  &suspend,
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: matchLabels},
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: matchLabels},
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyOnFailure,
							Containers:    []corev1.Container{ctr},
						},
					},
				},
			},
		},
	}

	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	created, err := cli.BatchV1().CronJobs(ns).Create(ctx, cj, metav1.CreateOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"namespace": created.Namespace,
			"name":      created.Name,
			"schedule":  created.Spec.Schedule,
			"suspend":   suspend,
		},
		"message": "CronJob created",
	}))
}

func (cc *controller) setCronJobSuspend(c fiber.Ctx, suspend bool) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	cj, err := cli.BatchV1().CronJobs(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	cj.Spec.Suspend = &suspend
	updated, err := cli.BatchV1().CronJobs(ns).Update(ctx, cj, metav1.UpdateOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	msg := "CronJob resumed"
	if suspend {
		msg = "CronJob suspended"
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"namespace": updated.Namespace,
			"name":      updated.Name,
			"suspend":   suspend,
		},
		"message": msg,
	}))
}

func (cc *controller) SuspendCronJobAPI(c fiber.Ctx) error {
	return cc.setCronJobSuspend(c, true)
}

func (cc *controller) ResumeCronJobAPI(c fiber.Ctx) error {
	return cc.setCronJobSuspend(c, false)
}

func (cc *controller) TriggerCronJobAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	cj, err := cli.BatchV1().CronJobs(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	jobName := fmt.Sprintf("%s-manual-%d", name, time.Now().Unix())
	if len(jobName) > 63 {
		jobName = jobName[:63]
	}
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: ns,
			Labels:    cj.Spec.JobTemplate.Labels,
			Annotations: map[string]string{
				"cronjob.kubernetes.io/instantiate": "manual",
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "batch/v1",
				Kind:       "CronJob",
				Name:       cj.Name,
				UID:        cj.UID,
				Controller: boolPtr(true),
			}},
		},
		Spec: cj.Spec.JobTemplate.Spec,
	}
	created, err := cli.BatchV1().Jobs(ns).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"namespace": created.Namespace,
			"name":      created.Name,
			"cronjob":   name,
		},
		"message": "Job triggered from CronJob",
	}))
}

func boolPtr(v bool) *bool { return &v }
