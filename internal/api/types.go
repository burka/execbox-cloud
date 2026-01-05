// Package api provides HTTP API types for execbox-cloud
package api

// CreateSessionRequest defines the request body for POST /v1/sessions
type CreateSessionRequest struct {
	Image     string            `json:"image"`
	Setup     []string          `json:"setup,omitempty"`  // RUN commands to bake into image
	Files     []FileSpec        `json:"files,omitempty"`  // Files to include in image
	Command   []string          `json:"command,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	WorkDir   string            `json:"workDir,omitempty"`
	Resources *Resources        `json:"resources,omitempty"`
	Network   string            `json:"network,omitempty"` // none|outgoing|exposed
	Ports     []PortSpec        `json:"ports,omitempty"`
}

// FileSpec defines a file to include in the built image
type FileSpec struct {
	Path     string `json:"path"`               // Destination path in container
	Content  string `json:"content"`            // File content (text or base64)
	Encoding string `json:"encoding,omitempty"` // "utf8" (default) or "base64"
}

// Resources defines resource limits for a session
type Resources struct {
	CPUMillis int `json:"cpuMillis,omitempty"` // CPU limit in millicores
	MemoryMB  int `json:"memoryMB,omitempty"`  // Memory limit in MB
	TimeoutMs int `json:"timeoutMs,omitempty"` // Timeout in milliseconds
}

// PortSpec defines a port to expose from the container
type PortSpec struct {
	Container int    `json:"container"`          // Container port number
	Protocol  string `json:"protocol,omitempty"` // tcp or udp (default: tcp)
}

// CreateSessionResponse defines the response body for POST /v1/sessions
type CreateSessionResponse struct {
	ID        string       `json:"id"`
	Status    string       `json:"status"`
	CreatedAt string       `json:"createdAt"`
	Network   *NetworkInfo `json:"network,omitempty"`
}

// NetworkInfo contains network configuration details for a session
type NetworkInfo struct {
	Mode  string              `json:"mode"`  // none|outgoing|exposed
	Host  string              `json:"host"`  // Hostname for accessing ports
	Ports map[string]PortInfo `json:"ports"` // Map of container port to host port info
}

// PortInfo contains details about an exposed port
type PortInfo struct {
	HostPort int    `json:"hostPort"` // Host port number
	URL      string `json:"url"`      // Full URL to access the port
}

// SessionResponse defines the response body for GET /v1/sessions/{id}
type SessionResponse struct {
	ID        string       `json:"id"`
	Status    string       `json:"status"`
	Image     string       `json:"image"`
	CreatedAt string       `json:"createdAt"`
	StartedAt *string      `json:"startedAt,omitempty"`
	EndedAt   *string      `json:"endedAt,omitempty"`
	ExitCode  *int         `json:"exitCode,omitempty"`
	Network   *NetworkInfo `json:"network,omitempty"`
}

// ListSessionsResponse defines the response body for GET /v1/sessions
type ListSessionsResponse struct {
	Sessions []SessionResponse `json:"sessions"`
}

// StopSessionResponse defines the response body for POST /v1/sessions/{id}/stop
type StopSessionResponse struct {
	Status string `json:"status"`
}

// GetURLResponse defines the response body for GET /v1/sessions/{id}/url
type GetURLResponse struct {
	ContainerPort int    `json:"containerPort"`
	HostPort      int    `json:"hostPort"`
	URL           string `json:"url"`
	Protocol      string `json:"protocol"`
}

// UploadFileResponse defines the response body for POST /v1/sessions/{id}/files
type UploadFileResponse struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// FileEntry represents a file or directory in a listing
type FileEntry struct {
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	IsDir bool   `json:"isDir"`
	Mode  uint32 `json:"mode"`
}

// ListDirectoryResponse defines the response body for GET /v1/sessions/{id}/files?list=true
type ListDirectoryResponse struct {
	Path    string      `json:"path"`
	Entries []FileEntry `json:"entries"`
}
