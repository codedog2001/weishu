package migrator

// 一条数据对应一个实体
type Entity interface {
	ID() int64
	//返回ID
	CompareTo(dst Entity) bool // 比较两个实体是否相同
}

//你要迁移哪张表 ，你就要去对应的dao层，找到对应的表，实现这个接口
