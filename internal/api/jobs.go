package api

import (
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	JobStatusPending    = "pending"
	JobStatusProcessing = "processing"
	JobStatusComplete   = "complete"
	JobStatusFailed     = "failed"

	FileStatusPending    = "pending"
	FileStatusProcessing = "processing"
	FileStatusComplete   = "complete"
	FileStatusError      = "error"
)

// UploadJob tracks the progress of an ingestion request across multiple files.
type UploadJob struct {
	ID        string           `json:"jobId"`
	Status    string           `json:"status"`
	CreatedAt time.Time        `json:"createdAt"`
	UpdatedAt time.Time        `json:"updatedAt"`
	Files     []FileProgress   `json:"files"`
	Results   []DocumentResult `json:"results,omitempty"`
	Error     string           `json:"error,omitempty"`
}

// FileProgress captures per-file progress updates that the frontend polls.
type FileProgress struct {
	Index   int             `json:"index"`
	Name    string          `json:"name"`
	Status  string          `json:"status"`
	Step    string          `json:"step,omitempty"`
	Message string          `json:"message,omitempty"`
	Current int             `json:"current"`
	Total   int             `json:"total"`
	Percent int             `json:"percent"`
	Result  *DocumentResult `json:"result,omitempty"`
	Error   string          `json:"error,omitempty"`
}

type JobManager struct {
	mu   sync.RWMutex
	jobs map[string]*UploadJob
}

func NewJobManager() *JobManager {
	return &JobManager{
		jobs: make(map[string]*UploadJob),
	}
}

func (m *JobManager) CreateJob(fileNames []string) (string, *UploadJob) {
	files := make([]FileProgress, len(fileNames))
	for i, name := range fileNames {
		files[i] = FileProgress{
			Index:  i,
			Name:   name,
			Status: FileStatusPending,
		}
	}
	job := &UploadJob{
		ID:        uuid.NewString(),
		Status:    JobStatusPending,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		Files:     files,
	}

	m.mu.Lock()
	m.jobs[job.ID] = job
	m.mu.Unlock()

	return job.ID, job.clone()
}

func (m *JobManager) GetJob(id string) (*UploadJob, bool) {
	m.mu.RLock()
	job, ok := m.jobs[id]
	m.mu.RUnlock()
	if !ok {
		return nil, false
	}
	return job.clone(), true
}

func (m *JobManager) MarkProcessing(id string) {
	m.withJob(id, func(job *UploadJob) {
		job.Status = JobStatusProcessing
	})
}

func (m *JobManager) MarkCompleted(id string) {
	m.withJob(id, func(job *UploadJob) {
		job.Status = JobStatusComplete
	})
}

func (m *JobManager) MarkFailed(id string, msg string) {
	m.withJob(id, func(job *UploadJob) {
		job.Status = JobStatusFailed
		job.Error = strings.TrimSpace(msg)
	})
}

func (m *JobManager) MarkFileStarted(id string, index int) {
	m.withJob(id, func(job *UploadJob) {
		if file := job.file(index); file != nil {
			file.Status = FileStatusProcessing
			file.Step = ""
			file.Message = "Starting"
			file.Current = 0
			file.Total = 100
			file.Percent = 0
			file.Error = ""
		}
	})
}

func (m *JobManager) UpdateFileProgress(id string, index int, step, message string, current, total int) {
	m.withJob(id, func(job *UploadJob) {
		if file := job.file(index); file != nil {
			file.Status = FileStatusProcessing
			file.Step = step
			file.Message = message
			file.Current = current
			file.Total = total
			file.Percent = percent(current, total)
		}
	})
}

func (m *JobManager) MarkFileComplete(id string, index int, result DocumentResult) {
	m.withJob(id, func(job *UploadJob) {
		if file := job.file(index); file != nil {
			file.Status = FileStatusComplete
			file.Step = "complete"
			file.Message = "Processing complete"
			file.Current = 100
			file.Total = 100
			file.Percent = 100
			file.Result = cloneResult(result)
			file.Error = ""
		}
		job.Results = append(job.Results, result)
	})
}

func (m *JobManager) MarkFileError(id string, index int, message string, result DocumentResult) {
	msg := strings.TrimSpace(message)
	if msg == "" {
		msg = "processing error"
	}
	m.withJob(id, func(job *UploadJob) {
		if file := job.file(index); file != nil {
			file.Status = FileStatusError
			file.Step = "error"
			file.Message = msg
			file.Error = msg
			file.Current = 100
			file.Total = 100
			file.Percent = 100
			file.Result = cloneResult(result)
		}
		result.Status = FileStatusError
		if result.Message == "" {
			result.Message = msg
		}
		job.Results = append(job.Results, result)
	})
}

func (m *JobManager) withJob(id string, fn func(job *UploadJob)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.jobs[id]
	if !ok {
		return
	}
	fn(job)
	job.UpdatedAt = time.Now().UTC()
}

func (job *UploadJob) file(index int) *FileProgress {
	if index < 0 || index >= len(job.Files) {
		return nil
	}
	return &job.Files[index]
}

func (job *UploadJob) clone() *UploadJob {
	if job == nil {
		return nil
	}
	copyJob := &UploadJob{
		ID:        job.ID,
		Status:    job.Status,
		CreatedAt: job.CreatedAt,
		UpdatedAt: job.UpdatedAt,
		Error:     job.Error,
	}
	if len(job.Files) > 0 {
		copyJob.Files = make([]FileProgress, len(job.Files))
		for i, file := range job.Files {
			copyJob.Files[i] = file
			if file.Result != nil {
				res := *file.Result
				copyJob.Files[i].Result = &res
			}
		}
	}
	if len(job.Results) > 0 {
		copyJob.Results = make([]DocumentResult, len(job.Results))
		for i, res := range job.Results {
			copyJob.Results[i] = res
		}
	}
	return copyJob
}

func cloneResult(result DocumentResult) *DocumentResult {
	res := result
	return &res
}

func percent(current, total int) int {
	if total <= 0 {
		if current <= 0 {
			return 0
		}
		if current > 100 {
			return 100
		}
		return current
	}
	if current <= 0 {
		return 0
	}
	if current >= total {
		return 100
	}
	return int((float64(current) / float64(total)) * 100)
}
