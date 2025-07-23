package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"mini-k8s-orchestration/pkg/types"
)

// Common errors
var (
	ErrResourceNotFound = errors.New("resource not found")
)

// Repository defines the interface for resource storage operations
type Repository interface {
	// Generic resource operations
	CreateResource(resource Resource) error
	GetResource(kind, namespace, name string) (Resource, error)
	UpdateResource(resource Resource) error
	DeleteResource(kind, namespace, name string) error
	ListResources(kind, namespace string) ([]Resource, error)

	// Node-specific operations
	CreateNode(node *types.Node) error
	GetNode(name string) (*types.Node, error)
	UpdateNode(node *types.Node) error
	DeleteNode(name string) error
	ListNodes() ([]*types.Node, error)
	UpdateNodeHeartbeat(name string) error

	// Pod assignment operations
	AssignPodToNode(podID, nodeID string) error
	GetPodAssignment(podID string) (*PodAssignment, error)
	UpdatePodAssignmentStatus(podID, status string) error
	DeletePodAssignment(podID string) error
	ListPodAssignmentsByNode(nodeID string) ([]*PodAssignment, error)
}

// Resource represents a generic Kubernetes resource
type Resource struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Namespace string    `json:"namespace"`
	Name      string    `json:"name"`
	Metadata  string    `json:"metadata"`  // JSON blob containing full metadata
	Spec      string    `json:"spec"`      // JSON blob
	Status    string    `json:"status"`    // JSON blob
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// PodAssignment represents a pod assignment to a node
type PodAssignment struct {
	PodID     string    `json:"podId"`
	NodeID    string    `json:"nodeId"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

// SQLRepository implements Repository using SQLite
type SQLRepository struct {
	db *Database
}

// NewSQLRepository creates a new SQL repository
func NewSQLRepository(db *Database) Repository {
	return &SQLRepository{db: db}
}

// CreateResource creates a new resource
func (r *SQLRepository) CreateResource(resource Resource) error {
	query := `
		INSERT INTO resources (id, kind, namespace, name, metadata, spec, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	
	now := time.Now()
	_, err := r.db.DB().Exec(query,
		resource.ID,
		resource.Kind,
		resource.Namespace,
		resource.Name,
		resource.Metadata,
		resource.Spec,
		resource.Status,
		now,
		now,
	)
	
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}
	
	return nil
}

// GetResource retrieves a resource by kind, namespace, and name
func (r *SQLRepository) GetResource(kind, namespace, name string) (Resource, error) {
	query := `
		SELECT id, kind, namespace, name, metadata, spec, status, created_at, updated_at
		FROM resources
		WHERE kind = ? AND namespace = ? AND name = ?
	`
	
	var resource Resource
	err := r.db.DB().QueryRow(query, kind, namespace, name).Scan(
		&resource.ID,
		&resource.Kind,
		&resource.Namespace,
		&resource.Name,
		&resource.Metadata,
		&resource.Spec,
		&resource.Status,
		&resource.CreatedAt,
		&resource.UpdatedAt,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return resource, fmt.Errorf("resource not found: %s/%s/%s", kind, namespace, name)
		}
		return resource, fmt.Errorf("failed to get resource: %w", err)
	}
	
	return resource, nil
}

// UpdateResource updates an existing resource
func (r *SQLRepository) UpdateResource(resource Resource) error {
	query := `
		UPDATE resources
		SET metadata = ?, spec = ?, status = ?, updated_at = ?
		WHERE kind = ? AND namespace = ? AND name = ?
	`
	
	result, err := r.db.DB().Exec(query,
		resource.Metadata,
		resource.Spec,
		resource.Status,
		time.Now(),
		resource.Kind,
		resource.Namespace,
		resource.Name,
	)
	
	if err != nil {
		return fmt.Errorf("failed to update resource: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("resource not found: %s/%s/%s", resource.Kind, resource.Namespace, resource.Name)
	}
	
	return nil
}

// DeleteResource deletes a resource
func (r *SQLRepository) DeleteResource(kind, namespace, name string) error {
	query := `DELETE FROM resources WHERE kind = ? AND namespace = ? AND name = ?`
	
	result, err := r.db.DB().Exec(query, kind, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to delete resource: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("resource not found: %s/%s/%s", kind, namespace, name)
	}
	
	return nil
}

// ListResources lists resources by kind and namespace
func (r *SQLRepository) ListResources(kind, namespace string) ([]Resource, error) {
	var query string
	var args []interface{}
	
	if namespace == "" {
		query = `
			SELECT id, kind, namespace, name, metadata, spec, status, created_at, updated_at
			FROM resources
			WHERE kind = ?
			ORDER BY created_at DESC
		`
		args = []interface{}{kind}
	} else {
		query = `
			SELECT id, kind, namespace, name, metadata, spec, status, created_at, updated_at
			FROM resources
			WHERE kind = ? AND namespace = ?
			ORDER BY created_at DESC
		`
		args = []interface{}{kind, namespace}
	}
	
	rows, err := r.db.DB().Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list resources: %w", err)
	}
	defer rows.Close()
	
	var resources []Resource
	for rows.Next() {
		var resource Resource
		err := rows.Scan(
			&resource.ID,
			&resource.Kind,
			&resource.Namespace,
			&resource.Name,
			&resource.Metadata,
			&resource.Spec,
			&resource.Status,
			&resource.CreatedAt,
			&resource.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan resource: %w", err)
		}
		resources = append(resources, resource)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}
	
	return resources, nil
}

// CreateNode creates a new node
func (r *SQLRepository) CreateNode(node *types.Node) error {
	statusJSON, err := json.Marshal(node.Status)
	if err != nil {
		return fmt.Errorf("failed to marshal node status: %w", err)
	}
	
	query := `
		INSERT INTO nodes (id, name, address, status, last_heartbeat, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	
	now := time.Now()
	address := ""
	if len(node.Status.Addresses) > 0 {
		address = node.Status.Addresses[0].Address
	}
	
	_, err = r.db.DB().Exec(query,
		node.Metadata.UID,
		node.Metadata.Name,
		address,
		string(statusJSON),
		now,
		now,
	)
	
	if err != nil {
		return fmt.Errorf("failed to create node: %w", err)
	}
	
	return nil
}

// GetNode retrieves a node by name
func (r *SQLRepository) GetNode(name string) (*types.Node, error) {
	query := `
		SELECT id, name, address, status, last_heartbeat, created_at
		FROM nodes
		WHERE name = ?
	`
	
	var id, nodeName, address, statusJSON string
	var lastHeartbeat, createdAt time.Time
	
	err := r.db.DB().QueryRow(query, name).Scan(
		&id,
		&nodeName,
		&address,
		&statusJSON,
		&lastHeartbeat,
		&createdAt,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("node not found: %s", name)
		}
		return nil, fmt.Errorf("failed to get node: %w", err)
	}
	
	var status types.NodeStatus
	if err := json.Unmarshal([]byte(statusJSON), &status); err != nil {
		return nil, fmt.Errorf("failed to unmarshal node status: %w", err)
	}
	
	node := &types.Node{
		APIVersion: "v1",
		Kind:       "Node",
		Metadata: types.ObjectMeta{
			Name:      nodeName,
			UID:       id,
			CreatedAt: createdAt,
		},
		Status: status,
	}
	
	return node, nil
}

// UpdateNode updates an existing node
func (r *SQLRepository) UpdateNode(node *types.Node) error {
	statusJSON, err := json.Marshal(node.Status)
	if err != nil {
		return fmt.Errorf("failed to marshal node status: %w", err)
	}
	
	address := ""
	if len(node.Status.Addresses) > 0 {
		address = node.Status.Addresses[0].Address
	}
	
	query := `
		UPDATE nodes
		SET address = ?, status = ?, last_heartbeat = ?
		WHERE name = ?
	`
	
	result, err := r.db.DB().Exec(query,
		address,
		string(statusJSON),
		time.Now(),
		node.Metadata.Name,
	)
	
	if err != nil {
		return fmt.Errorf("failed to update node: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("node not found: %s", node.Metadata.Name)
	}
	
	return nil
}

// DeleteNode deletes a node
func (r *SQLRepository) DeleteNode(name string) error {
	query := `DELETE FROM nodes WHERE name = ?`
	
	result, err := r.db.DB().Exec(query, name)
	if err != nil {
		return fmt.Errorf("failed to delete node: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("node not found: %s", name)
	}
	
	return nil
}

// ListNodes lists all nodes
func (r *SQLRepository) ListNodes() ([]*types.Node, error) {
	query := `
		SELECT id, name, address, status, last_heartbeat, created_at
		FROM nodes
		ORDER BY created_at DESC
	`
	
	rows, err := r.db.DB().Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}
	defer rows.Close()
	
	var nodes []*types.Node
	for rows.Next() {
		var id, nodeName, address, statusJSON string
		var lastHeartbeat, createdAt time.Time
		
		err := rows.Scan(
			&id,
			&nodeName,
			&address,
			&statusJSON,
			&lastHeartbeat,
			&createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan node: %w", err)
		}
		
		var status types.NodeStatus
		if err := json.Unmarshal([]byte(statusJSON), &status); err != nil {
			return nil, fmt.Errorf("failed to unmarshal node status: %w", err)
		}
		
		node := &types.Node{
			APIVersion: "v1",
			Kind:       "Node",
			Metadata: types.ObjectMeta{
				Name:      nodeName,
				UID:       id,
				CreatedAt: createdAt,
			},
			Status: status,
		}
		
		nodes = append(nodes, node)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}
	
	return nodes, nil
}

// UpdateNodeHeartbeat updates the last heartbeat time for a node
func (r *SQLRepository) UpdateNodeHeartbeat(name string) error {
	query := `UPDATE nodes SET last_heartbeat = ? WHERE name = ?`
	
	result, err := r.db.DB().Exec(query, time.Now(), name)
	if err != nil {
		return fmt.Errorf("failed to update node heartbeat: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("node not found: %s", name)
	}
	
	return nil
}

// AssignPodToNode assigns a pod to a node
func (r *SQLRepository) AssignPodToNode(podID, nodeID string) error {
	query := `
		INSERT INTO pod_assignments (pod_id, node_id, status, created_at)
		VALUES (?, ?, ?, ?)
	`
	
	_, err := r.db.DB().Exec(query, podID, nodeID, "Pending", time.Now())
	if err != nil {
		return fmt.Errorf("failed to assign pod to node: %w", err)
	}
	
	return nil
}

// GetPodAssignment retrieves a pod assignment
func (r *SQLRepository) GetPodAssignment(podID string) (*PodAssignment, error) {
	query := `
		SELECT pod_id, node_id, status, created_at
		FROM pod_assignments
		WHERE pod_id = ?
	`
	
	var assignment PodAssignment
	err := r.db.DB().QueryRow(query, podID).Scan(
		&assignment.PodID,
		&assignment.NodeID,
		&assignment.Status,
		&assignment.CreatedAt,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("pod assignment not found: %s", podID)
		}
		return nil, fmt.Errorf("failed to get pod assignment: %w", err)
	}
	
	return &assignment, nil
}

// UpdatePodAssignmentStatus updates the status of a pod assignment
func (r *SQLRepository) UpdatePodAssignmentStatus(podID, status string) error {
	query := `UPDATE pod_assignments SET status = ? WHERE pod_id = ?`
	
	result, err := r.db.DB().Exec(query, status, podID)
	if err != nil {
		return fmt.Errorf("failed to update pod assignment status: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("pod assignment not found: %s", podID)
	}
	
	return nil
}

// DeletePodAssignment deletes a pod assignment
func (r *SQLRepository) DeletePodAssignment(podID string) error {
	query := `DELETE FROM pod_assignments WHERE pod_id = ?`
	
	result, err := r.db.DB().Exec(query, podID)
	if err != nil {
		return fmt.Errorf("failed to delete pod assignment: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("pod assignment not found: %s", podID)
	}
	
	return nil
}

// ListPodAssignmentsByNode lists all pod assignments for a specific node
func (r *SQLRepository) ListPodAssignmentsByNode(nodeID string) ([]*PodAssignment, error) {
	query := `
		SELECT pod_id, node_id, status, created_at
		FROM pod_assignments
		WHERE node_id = ?
		ORDER BY created_at DESC
	`
	
	rows, err := r.db.DB().Query(query, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to list pod assignments: %w", err)
	}
	defer rows.Close()
	
	var assignments []*PodAssignment
	for rows.Next() {
		var assignment PodAssignment
		err := rows.Scan(
			&assignment.PodID,
			&assignment.NodeID,
			&assignment.Status,
			&assignment.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan pod assignment: %w", err)
		}
		assignments = append(assignments, &assignment)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}
	
	return assignments, nil
}