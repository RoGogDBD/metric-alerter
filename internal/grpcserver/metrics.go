package grpcserver

import (
	"fmt"

	"github.com/RoGogDBD/metric-alerter/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MetricsService реализует gRPC сервис для обновления метрик.
type MetricsService struct {
	proto.UnimplementedMetricsServer
	storage repository.Storage
	db      *pgxpool.Pool
}

// NewMetricsService создает новый gRPC сервис метрик.
func NewMetricsService(storage repository.Storage, db *pgxpool.Pool) *MetricsService {
	return &MetricsService{storage: storage, db: db}
}

// UpdateMetrics обновляет метрики на сервере.
func (s *MetricsService) UpdateMetrics(ctx context.Context, req *proto.UpdateMetricsRequest) (*proto.UpdateMetricsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	for _, metric := range req.GetMetrics() {
		if metric.GetId() == "" {
			return nil, status.Error(codes.InvalidArgument, "metric id is required")
		}
		switch metric.GetType() {
		case proto.Metric_GAUGE:
			s.storage.SetGauge(metric.GetId(), metric.GetValue())
		case proto.Metric_COUNTER:
			s.storage.AddCounter(metric.GetId(), metric.GetDelta())
		default:
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("unknown metric type: %v", metric.GetType()))
		}
	}

	if s.db != nil {
		if err := repository.SyncToDB(ctx, s.storage, s.db); err != nil {
			return nil, status.Error(codes.Internal, "failed to save metrics")
		}
	}

	return &proto.UpdateMetricsResponse{}, nil
}
