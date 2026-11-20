// Package storage 提供巡检数据的持久化能力。
// Store 是存储接口，便于将来替换存储实现或在测试中打桩。
package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/enmotplinux/onsite-inspection/internal/config"
	"github.com/enmotplinux/onsite-inspection/internal/model"
)

// Store 存储接口。
type Store interface {
	// Close 释放连接资源。
	Close(ctx context.Context) error

	// ListInstances 返回全部巡检目标实例。
	ListInstances(ctx context.Context) ([]model.Instance, error)

	// AddInstance 新增或更新实例（按 ID upsert）。
	AddInstance(ctx context.Context, inst model.Instance) error

	// RemoveInstance 删除实例，返回是否真的删除了记录。
	RemoveInstance(ctx context.Context, id string) (bool, error)

	// SaveResult 写入一次巡检结果，并按保留策略清理该实例的历史结果。
	SaveResult(ctx context.Context, r model.InspectionResult, retention int) error

	// LatestResult 返回实例最近一次巡检结果，无结果时返回 nil。
	LatestResult(ctx context.Context, instanceID string) (*model.InspectionResult, error)
}

const (
	collectionInstances = "instances"
	collectionResults    = "results"
)

// MongoStore 基于 MongoDB 的存储实现。
// 底层 client 自带连接池，全程复用，替代旧版"每次操作重建连接"的模式。
type MongoStore struct {
	client *mongo.Client
	db     *mongo.Database
}

// NewMongoStore 建立 MongoDB 连接（含连通性验证）。
func NewMongoStore(ctx context.Context, cfg config.MongoDBConfig) (*MongoStore, error) {
	opts := options.Client().
		ApplyURI(cfg.URI()).
		SetConnectTimeout(5 * time.Second).
		SetServerSelectionTimeout(5 * time.Second)

	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("连接 MongoDB: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(ctx)
		// 错误信息只暴露地址，不暴露带凭据的完整 URI
		return nil, fmt.Errorf("MongoDB 连通性验证失败 (%s:%d): %w", cfg.Host, cfg.Port, err)
	}
	s := &MongoStore{client: client, db: client.Database(cfg.Database)}
	if err := s.ensureIndexes(ctx); err != nil {
		_ = client.Disconnect(ctx)
		return nil, err
	}
	return s, nil
}

// ensureIndexes 创建查询热路径索引（幂等，已存在时为 no-op）：
// results 按 (instance_id, collected_at desc) 查询最新结果与清理历史。
func (s *MongoStore) ensureIndexes(ctx context.Context) error {
	_, err := s.db.Collection(collectionResults).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "instance_id", Value: 1},
			{Key: "collected_at", Value: -1},
		},
	})
	if err != nil {
		return fmt.Errorf("创建 results 索引: %w", err)
	}
	return nil
}

// Close 释放 MongoDB 连接。
func (s *MongoStore) Close(ctx context.Context) error {
	return s.client.Disconnect(ctx)
}

// ListInstances 列出全部实例，按 ID 排序保证输出稳定。
func (s *MongoStore) ListInstances(ctx context.Context) ([]model.Instance, error) {
	cur, err := s.db.Collection(collectionInstances).
		Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("查询实例列表: %w", err)
	}
	defer cur.Close(ctx)

	var out []model.Instance
	if err := cur.All(ctx, &out); err != nil {
		return nil, fmt.Errorf("解码实例列表: %w", err)
	}
	return out, nil
}

// AddInstance 按 ID upsert 实例信息。
func (s *MongoStore) AddInstance(ctx context.Context, inst model.Instance) error {
	now := time.Now()
	inst.UpdatedAt = now

	filter := bson.M{"_id": inst.ID}
	update := bson.M{
		"$set": bson.M{
			"host":       inst.Host,
			"port":       inst.Port,
			"user":       inst.User,
			"password":   inst.Password,
			"updated_at": inst.UpdatedAt,
		},
		"$setOnInsert": bson.M{"created_at": now},
	}
	_, err := s.db.Collection(collectionInstances).UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("写入实例 %s: %w", inst.ID, err)
	}
	return nil
}

// RemoveInstance 删除实例，返回是否实际删除。
func (s *MongoStore) RemoveInstance(ctx context.Context, id string) (bool, error) {
	res, err := s.db.Collection(collectionInstances).DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return false, fmt.Errorf("删除实例 %s: %w", id, err)
	}
	return res.DeletedCount > 0, nil
}

// SaveResult 写入巡检结果并按 retention 清理历史。
func (s *MongoStore) SaveResult(ctx context.Context, r model.InspectionResult, retention int) error {
	col := s.db.Collection(collectionResults)
	if _, err := col.InsertOne(ctx, r); err != nil {
		return fmt.Errorf("写入实例 %s 巡检结果: %w", r.InstanceID, err)
	}
	return s.trimResults(ctx, col, r.InstanceID, retention)
}

// trimResults 只保留每个实例最近 retention 份结果。
// 实现：按时间倒序跳过前 retention-1 条，找到第 retention 条的时间戳，
// 删除所有早于该时间戳的记录。
func (s *MongoStore) trimResults(ctx context.Context, col *mongo.Collection, instanceID string, retention int) error {
	findOpts := options.Find().
		SetSort(bson.D{{Key: "collected_at", Value: -1}}).
		SetSkip(int64(retention - 1)).
		SetLimit(1).
		SetProjection(bson.M{"collected_at": 1})

	cur, err := col.Find(ctx, bson.M{"instance_id": instanceID}, findOpts)
	if err != nil {
		return fmt.Errorf("查询实例 %s 历史结果: %w", instanceID, err)
	}
	defer cur.Close(ctx)

	var boundary struct {
		CollectedAt time.Time `bson:"collected_at"`
	}
	if !cur.Next(ctx) {
		return nil // 结果总数不足 retention 份，无需清理
	}
	if err := cur.Decode(&boundary); err != nil {
		return fmt.Errorf("解码历史结果边界: %w", err)
	}

	_, err = col.DeleteMany(ctx, bson.M{
		"instance_id":  instanceID,
		"collected_at": bson.M{"$lt": boundary.CollectedAt},
	})
	if err != nil {
		return fmt.Errorf("清理实例 %s 历史结果: %w", instanceID, err)
	}
	return nil
}

// LatestResult 返回实例最近一次巡检结果。
func (s *MongoStore) LatestResult(ctx context.Context, instanceID string) (*model.InspectionResult, error) {
	findOpts := options.FindOne().SetSort(bson.D{{Key: "collected_at", Value: -1}})
	var r model.InspectionResult
	err := s.db.Collection(collectionResults).FindOne(ctx, bson.M{"instance_id": instanceID}, findOpts).Decode(&r)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("查询实例 %s 巡检结果: %w", instanceID, err)
	}
	return &r, nil
}
