//go:build wireinject

package startup

import (
	"github.com/google/wire"
)

func InitServer() *grpc.FollowServiceServer {
	wire.Build(
		InitRedis,
		InitLog,
		InitTestDB,
		dao.NewGORMFollowRelationDAO,
		cache.NewRedisFollowCache,
		repository.NewFollowRelationRepository,
		service.NewFollowRelationService,
		grpc.NewFollowRelationServiceServer,
	)
	return new(grpc.FollowServiceServer)
}
