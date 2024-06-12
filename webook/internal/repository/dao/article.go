package dao

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type Article struct {
	Id      int64  `gorm:"primaryKey,autoIncrement" bson:"id,omitempty"`
	Title   string `gorm:"type=varchar(4096)" bson:"title,omitempty"`
	Content string `gorm:"type=BLOB" bson:"content,omitempty"`
	// 我要根据创作者ID来查询
	AuthorId int64 `gorm:"index" bson:"author_id,omitempty"`
	Status   uint8 `bson:"status,omitempty"`
	Ctime    int64 `bson:"ctime,omitempty"`
	// 更新时间
	Utime int64 `bson:"utime,omitempty"`
}
type ArticleDAO interface {
	Insert(ctx context.Context, art Article) (int64, error)
	UpdateById(ctx context.Context, entity Article) error
	Sync(ctx context.Context, entity Article) (int64, error)
	SyncStatus(ctx context.Context, uid int64, id int64, status uint8) error
	GetByAuthor(ctx context.Context, uid int64, offset int, limit int) ([]Article, error)
	GetById(ctx context.Context, id int64) (Article, error)
	GetPubById(ctx context.Context, id int64) (PublishedArticle, error)
	ListPub(ctx context.Context, start time.Time, offset int, limit int) ([]PublishedArticle, error)
	GetTopArticles(ctx context.Context, biz string, number int) (map[string]int64, error)
}

type ArticleGORMDAO struct {
	db *gorm.DB
}

func (a *ArticleGORMDAO) GetTopArticles(ctx context.Context, biz string, number int) (map[string]int64, error) {
	var results []struct {
		BizID string `gorm:"column:bizid"`
		Likes int64  `gorm:"column:likes"`
	}
	err := a.db.WithContext(ctx).Raw("SELECT bizid, likes FROM articles WHERE biz = ? ORDER BY likes DESC LIMIT ?", biz, number).Scan(&results).Error
	if err != nil {
		return nil, err
	}

	articles := make(map[string]int64)
	for _, result := range results {
		articles[result.BizID] = result.Likes
	}
	return articles, nil
}

func (a *ArticleGORMDAO) ListPub(ctx context.Context, start time.Time, offset int, limit int) ([]PublishedArticle, error) {
	var res []PublishedArticle
	const ArticleStatusPublished = 2
	err := a.db.WithContext(ctx).
		Where("utime < ? AND status = ?",
			start.UnixMilli(), ArticleStatusPublished).
		Offset(offset).Limit(limit).
		Find(&res).Error
	return res, err
}

func (a ArticleGORMDAO) Insert(ctx context.Context, art Article) (int64, error) {
	now := time.Now().UnixMilli()
	art.Utime = now
	art.Ctime = now
	err := a.db.WithContext(ctx).Create(&art).Error
	return art.Id, err
}

func (a ArticleGORMDAO) UpdateById(ctx context.Context, art Article) error {
	now := time.Now().UnixMilli()
	//这里是数据操作必须文章id和作者id都需要命中，否则不会更新，就保证了避免别人乱更新文章的问题
	res := a.db.WithContext(ctx).Model(&art).
		Where("id=? AND author_id=?", art.Id, art.AuthorId).Updates(map[string]interface{}{
		"title":   art.Title,
		"content": art.Content,
		"utime":   now,
		"status":  art.Status,
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("ID不对或者创作者不对")
	}
	return nil
}

func (a ArticleGORMDAO) Sync(ctx context.Context, art Article) (int64, error) {
	var id = art.Id
	//开启事务
	err := a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var (
			err error
		)
		dao := NewArticleGORMDAO(tx)
		if id > 0 {
			//更新,说明该文章已经存在
			err = dao.UpdateById(ctx, art)
		} else {
			//插入，该文章不存在，所以是插入
			id, err = dao.Insert(ctx, art)
		}
		if err != nil {
			return err
		}
		//上面都是在更新或者插入制作库，
		//下面开始操作线上库
		art.Id = id
		now := time.Now().UnixMilli()
		pubArt := PublishedArticle(art)
		pubArt.Ctime = now
		pubArt.Utime = now
		err = tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"title":   pubArt.Title,
				"content": pubArt.Content,
				"utime":   now,
				"status":  pubArt.Status,
			}),
		}).Create(&pubArt).Error
		return err
		//线上库一样是有两种可能，
		//一种是已经同步到线上库，这时候只需要更新相应的字段即可，比如说已经发表的文章修改后重新发表
		//第二种是之前没有同步到线上库的文章，这种情况下，需要插入线上库
	})
	return id, err

}

// 事务具有四个基本特性，通常被称为ACID属性：
//
// 原子性（Atomicity）：事务是一个原子操作单元，其对数据的修改要么全都执行，要么全都不执行。
//
// 一致性（Consistency）：事务必须使数据库从一个一致性状态变换到另一个一致性状态。
//
// 隔离性（Isolation）：事务的执行不受其他事务的干扰，事务执行的中间结果对其他事务是不可见的。
//
// 持久性（Durability）：一旦事务提交，则其结果就是永久性的，即使系统崩溃也不会丢失。
func (a ArticleGORMDAO) SyncV1(ctx context.Context, art Article) (int64, error) {
	tx := a.db.WithContext(ctx).Begin() //手动开启事务
	if tx.Error != nil {
		return 0, tx.Error
	}
	defer tx.Rollback() //回滚，当事务中某一步出错之后，就可以撤销已经操作的步骤
	var (
		id  = art.Id
		err error
	)
	dao := NewArticleGORMDAO(tx)
	//先操作制作库
	if id > 0 {
		//更新,说明该文章已经存在
		err = dao.UpdateById(ctx, art)
	} else {
		id, err = dao.Insert(ctx, art)
	}
	if err != nil {
		return 0, err
	}
	art.Id = id
	now := time.Now().UnixMilli()
	pubArt := PublishedArticle(art)
	pubArt.Ctime = now
	pubArt.Utime = now
	err = tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"title":   pubArt.Title,
			"content": pubArt.Content,
			"utime":   pubArt.Utime,
		}),
	}).Create(&pubArt).Error
	if err != nil {
		return 0, err
	}
	tx.Commit() //提交事务，当事务中所有事情都完成后，就会把结果保存到数据库中，保持acid一致性
	return id, nil
}

func (a ArticleGORMDAO) SyncStatus(ctx context.Context, uid int64, id int64, status uint8) error {
	//查表改状态数据，把制作库和线上库的都改了，开事务处理
	now := time.Now().UnixMilli()
	return a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&Article{}).Where("id=? AND author_id", id, uid).
			Updates(map[string]any{
				"utime":  now,
				"status": status,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errors.New("ID不对或者创作者不对")
		}
		return tx.Model(&PublishedArticle{}).Where("id=? AND author_id=?", id, uid).
			Updates(map[string]any{
				"utime":  now,
				"status": status,
			}).Error
	})

}

func (a ArticleGORMDAO) GetByAuthor(ctx context.Context, uid int64, offset int, limit int) ([]Article, error) {
	panic("implement me")
}

func (a ArticleGORMDAO) GetById(ctx context.Context, id int64) (Article, error) {
	var art Article
	err := a.db.WithContext(ctx).Model(&Article{}).Where("id=?", id).First(&art).Error
	if err != nil {
		return Article{}, err
	}
	return art, nil

}

func (a ArticleGORMDAO) GetPubById(ctx context.Context, id int64) (PublishedArticle, error) {
	var res PublishedArticle
	err := a.db.WithContext(ctx).Where("id=?", id).First(&res).Error
	return res, err
}

func NewArticleGORMDAO(db *gorm.DB) ArticleDAO {
	return &ArticleGORMDAO{
		db: db,
	}
}

// 制作库的内容和线上库的内容保持一样，方便同步
type PublishedArticle Article
