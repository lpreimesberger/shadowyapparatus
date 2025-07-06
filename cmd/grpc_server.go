package cmd

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ShadowService represents the gRPC service implementation
type ShadowService struct {
	node *ShadowNode
}

// initializeGRPCServer sets up the gRPC server
func (sn *ShadowNode) initializeGRPCServer() error {
	// Create gRPC server
	sn.grpcServer = grpc.NewServer()
	
	// Register services
	_ = &ShadowService{node: sn}
	
	// TODO: Register proto-generated services here
	// Example: RegisterShadowServiceServer(sn.grpcServer, service)
	
	sn.updateHealthStatus("grpc", "healthy", nil, map[string]interface{}{
		"port": sn.config.GRPCPort,
	})
	
	return nil
}

// startGRPCServer starts the gRPC server (called from node.go)
func (sn *ShadowNode) startGRPCServer() error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", sn.config.GRPCPort))
	if err != nil {
		return fmt.Errorf("failed to listen on gRPC port %d: %w", sn.config.GRPCPort, err)
	}
	
	return sn.grpcServer.Serve(listener)
}

// Example gRPC service methods (would be generated from .proto files)

// GetNodeStatus returns the status of the node
func (s *ShadowService) GetNodeStatus(ctx context.Context, req *NodeStatusRequest) (*NodeStatusResponse, error) {
	if s.node == nil {
		return nil, status.Error(codes.Internal, "node not available")
	}
	
	health := s.node.GetHealthStatus()
	
	// Convert health status to protobuf response
	// This would normally be generated from .proto files
	response := &NodeStatusResponse{
		NodeId:     "shadowy-node-1",
		Version:    "0.1.0",
		Healthy:    true,
		Services:   make(map[string]*ServiceStatus),
	}
	
	for name, status := range health {
		response.Services[name] = &ServiceStatus{
			Name:      status.Name,
			Status:    status.Status,
			LastCheck: status.LastCheck.Unix(),
			Error:     status.Error,
		}
		
		if status.Status != "healthy" {
			response.Healthy = false
		}
	}
	
	return response, nil
}

// SubmitTransaction submits a transaction via gRPC
func (s *ShadowService) SubmitTransaction(ctx context.Context, req *SubmitTransactionRequest) (*SubmitTransactionResponse, error) {
	if s.node.mempool == nil {
		return nil, status.Error(codes.Unavailable, "mempool not available")
	}
	
	// Convert protobuf request to internal types
	// This would normally be generated from .proto files
	signedTx := &SignedTransaction{
		TxHash:    req.Transaction.Hash,
		Algorithm: req.Transaction.Algorithm,
		// ... other fields
	}
	
	err := s.node.mempool.AddTransaction(signedTx, SourceNetwork)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("failed to add transaction: %v", err))
	}
	
	return &SubmitTransactionResponse{
		Status:  "accepted",
		TxHash:  signedTx.TxHash,
		Message: "Transaction added to mempool",
	}, nil
}

// GetMempoolStats returns mempool statistics
func (s *ShadowService) GetMempoolStats(ctx context.Context, req *MempoolStatsRequest) (*MempoolStatsResponse, error) {
	if s.node.mempool == nil {
		return nil, status.Error(codes.Unavailable, "mempool not available")
	}
	
	stats := s.node.mempool.GetStats()
	
	return &MempoolStatsResponse{
		TransactionCount: int32(stats.TransactionCount),
		TotalSize:       stats.TotalSize,
		AverageFee:      stats.AverageFee,
		ValidTxs:        int32(stats.ValidationStats.ValidTransactions),
		InvalidTxs:      int32(stats.ValidationStats.InvalidTransactions),
	}, nil
}

// SubmitVDFJob submits a VDF job via gRPC
func (s *ShadowService) SubmitVDFJob(ctx context.Context, req *SubmitVDFJobRequest) (*SubmitVDFJobResponse, error) {
	if s.node.timelord == nil {
		return nil, status.Error(codes.Unavailable, "timelord not available")
	}
	
	job, err := s.node.timelord.SubmitChallenge(req.Data, int(req.Priority))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("failed to submit VDF job: %v", err))
	}
	
	return &SubmitVDFJobResponse{
		Status: "accepted",
		JobId:  job.ID,
	}, nil
}

// GetVDFJob retrieves a VDF job status
func (s *ShadowService) GetVDFJob(ctx context.Context, req *GetVDFJobRequest) (*GetVDFJobResponse, error) {
	if s.node.timelord == nil {
		return nil, status.Error(codes.Unavailable, "timelord not available")
	}
	
	job, err := s.node.timelord.GetJob(req.JobId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "job not found")
	}
	
	response := &GetVDFJobResponse{
		JobId:       job.ID,
		Status:      job.Status.String(),
		Priority:    int32(job.Priority),
		SubmittedAt: job.SubmittedAt.Unix(),
	}
	
	if job.StartedAt != nil {
		response.StartedAt = job.StartedAt.Unix()
	}
	
	if job.CompletedAt != nil {
		response.CompletedAt = job.CompletedAt.Unix()
	}
	
	if job.Result != nil {
		response.IsValid = job.Result.IsValid
		response.Error = job.Result.Error
	}
	
	return response, nil
}

// Placeholder types (would normally be generated from .proto files)
type NodeStatusRequest struct{}

type NodeStatusResponse struct {
	NodeId   string                    `json:"node_id"`
	Version  string                    `json:"version"`
	Healthy  bool                      `json:"healthy"`
	Services map[string]*ServiceStatus `json:"services"`
}

type ServiceStatus struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	LastCheck int64  `json:"last_check"`
	Error     string `json:"error,omitempty"`
}

type SubmitTransactionRequest struct {
	Transaction *GRPCTransaction `json:"transaction"`
}

type GRPCTransaction struct {
	Hash      string `json:"hash"`
	Algorithm string `json:"algorithm"`
	// ... other fields
}

type SubmitTransactionResponse struct {
	Status  string `json:"status"`
	TxHash  string `json:"tx_hash"`
	Message string `json:"message"`
}

type MempoolStatsRequest struct{}

type MempoolStatsResponse struct {
	TransactionCount int32  `json:"transaction_count"`
	TotalSize       int64  `json:"total_size"`
	AverageFee      uint64 `json:"average_fee"`
	ValidTxs        int32  `json:"valid_txs"`
	InvalidTxs      int32  `json:"invalid_txs"`
}

type SubmitVDFJobRequest struct {
	Data     []byte `json:"data"`
	Priority int32  `json:"priority"`
}

type SubmitVDFJobResponse struct {
	Status string `json:"status"`
	JobId  string `json:"job_id"`
}

type GetVDFJobRequest struct {
	JobId string `json:"job_id"`
}

type GetVDFJobResponse struct {
	JobId       string `json:"job_id"`
	Status      string `json:"status"`
	Priority    int32  `json:"priority"`
	SubmittedAt int64  `json:"submitted_at"`
	StartedAt   int64  `json:"started_at,omitempty"`
	CompletedAt int64  `json:"completed_at,omitempty"`
	IsValid     bool   `json:"is_valid"`
	Error       string `json:"error,omitempty"`
}

// TODO: Create .proto files and generate proper gRPC services
// Example proto structure:
/*
syntax = "proto3";

package shadowy;

service ShadowService {
  rpc GetNodeStatus(NodeStatusRequest) returns (NodeStatusResponse);
  rpc SubmitTransaction(SubmitTransactionRequest) returns (SubmitTransactionResponse);
  rpc GetMempoolStats(MempoolStatsRequest) returns (MempoolStatsResponse);
  rpc SubmitVDFJob(SubmitVDFJobRequest) returns (SubmitVDFJobResponse);
  rpc GetVDFJob(GetVDFJobRequest) returns (GetVDFJobResponse);
}

message NodeStatusRequest {}
message NodeStatusResponse {
  string node_id = 1;
  string version = 2;
  bool healthy = 3;
  map<string, ServiceStatus> services = 4;
}

// ... other message definitions
*/