package web

import (
	"github.com/ecodeclub/ekit/slice"
	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
	"net/http"
	"strconv"
	"time"
	intrv1 "xiaoweishu/webook/api/proto/gen/intr/v1"
	"xiaoweishu/webook/internal/domain"
	"xiaoweishu/webook/internal/service"
	ijwt "xiaoweishu/webook/internal/web/jwt"
	logger2 "xiaoweishu/webook/pkg/logger"
)

//只有web层不需要用到接口，再往下都应该定义成接口

type ArticleHandler struct {
	svc     service.ArticleService
	l       logger2.LoggerV1
	biz     string //这个标识是为了跟视频，图片等业务进行区分
	intrSvc intrv1.InteractiveServiceClient
}

func NewArticleHandler(l logger2.LoggerV1,
	svc service.ArticleService,
	intrSvc intrv1.InteractiveServiceClient) *ArticleHandler {
	return &ArticleHandler{
		svc:     svc,
		l:       l,
		intrSvc: intrSvc,
		biz:     "article",
	}
}
func (h *ArticleHandler) RegisterRoutes(server *gin.Engine) {
	g := server.Group("/articles")
	//创作者接口
	g.POST("/publish", h.Publish)
	g.POST("/edit", h.Edit)
	g.POST("/withdraw", h.Withdraw)
	//这是创作者接口的查看详情
	g.GET("/detail/:id", h.Detail)
	//查看某作者的所有文章列表
	g.POST("/list", h.List)
	//读者接口
	pub := g.Group("/pub")
	pub.GET("/:id", h.PubDetail)
	pub.POST("/like", h.Like)
	pub.POST("/collect", h.Collect)
	pub.POST("/like100", h.Like100)

}

// 时刻记住web层/handler层是要跟前端打交道的，要的数据都 可以从前端去拿
func (h *ArticleHandler) Publish(ctx *gin.Context) {
	type Req struct {
		Id      int64
		Title   string
		Content string
	}
	var req Req
	err := ctx.Bind(&req)
	if err != nil {
		return
	}
	uc := ctx.MustGet("claims").(*ijwt.UserClaims)
	id, err := h.svc.Publish(ctx, domain.Article{
		Id:      req.Id,
		Title:   req.Title,
		Content: req.Content,
		Author: domain.Author{
			Id: uc.Uid,
		},
	})
	if err != nil {
		ctx.JSON(http.StatusOK, Result{
			Msg:  "系统错误",
			Code: 5,
		})
		h.l.Error("发表文章失败",
			logger2.Int64("uid", uc.Uid),
			logger2.Error(err))
		return
	}
	ctx.JSON(http.StatusOK, Result{
		Data: id,
		//成功就把文章id给返回回去
	})

}

func (h *ArticleHandler) Edit(ctx *gin.Context) {
	type Req struct {
		Id      int64
		Content string
		Title   string
	}
	var req Req
	err := ctx.Bind(&req)
	if err != nil {
		return
	}
	uc := ctx.MustGet("user").(ijwt.UserClaims)
	id, err := h.svc.Save(ctx, domain.Article{
		Id:      req.Id,
		Content: req.Content,
		Title:   req.Title,
		Author: domain.Author{
			Id: uc.Uid,
		},
	})
	if err != nil {
		ctx.JSON(http.StatusOK, Result{
			Msg:  "系统错误",
			Code: 5,
		})
		h.l.Error("保存文章数据失败",
			logger2.Int64("uid", uc.Uid),
			logger2.Error(err))
		return
	}
	ctx.JSON(http.StatusOK, Result{
		Data: id,
	})

}

func (h *ArticleHandler) Withdraw(ctx *gin.Context) {
	type Req struct {
		Id int64
	}
	var req Req
	err := ctx.Bind(&req)
	if err != nil {
		return
	}
	uc := ctx.MustGet("user").(ijwt.UserClaims)
	err = h.svc.Withdraw(ctx, uc.Uid, req.Id)
	if err != nil {
		ctx.JSON(http.StatusOK, Result{
			Code: 5,
			Msg:  "撤回文章失败",
		})
		h.l.Error("撤回文章失败",
			logger2.Int64("uid", uc.Uid),
			logger2.Int64("id", req.Id),
			logger2.Error(err))
		return
	}
	ctx.JSON(http.StatusOK, Result{
		Msg: "ok"})
}

func (h *ArticleHandler) Detail(ctx *gin.Context) {
	//路径传参，从ctx中取出id,不过是string类型，注意类型转换成int64
	idstr := ctx.Param("id")
	id, err := strconv.ParseInt(idstr, 10, 64)
	if err != nil {
		return
	} //转换成10进制的int64类型
	//根据id查文章，并将文章返回到art中
	art, err := h.svc.GetById(ctx, id)
	if err != nil {
		ctx.JSON(http.StatusOK, Result{
			Msg: "系统错误", //查看文章详情失败
		})
		h.l.Error("查看文章详情失败", logger2.Int64("id", id))
		return
	}
	//接下来做一个鉴权，因为这是创作者接口的查询，按道理来说，是不能查询别人的文章的
	uc := ctx.MustGet("user").(ijwt.UserClaims)
	if uc.Uid != art.Author.Id {
		ctx.JSON(http.StatusOK, Result{
			Msg:  "系统错误", //无权查看他人文章
			Code: 5,
		})
		h.l.Error("非法查询文章",
			logger2.Int64("id", id),
			logger2.Int64("uid", uc.Uid))
		return
	}
	//最后需要新建一个结构体，将对应的字段展示给前端
	vo := ArticleVo{
		Id:       art.Id,
		Title:    art.Title,
		Content:  art.Content,
		AuthorId: art.Author.Id, //这个字段没有也行，创作者不会在意自己的uid
		Status:   art.Status.ToUint8(),
		//这是给前端交互的，所以不能直接设置成time.time,需要转换成string
		Ctime: art.Ctime.Format(time.DateTime),
		Utime: art.Utime.Format(time.DateTime),
	}
	ctx.JSON(http.StatusOK, Result{
		Data: vo,
	})
}

func (h *ArticleHandler) List(ctx *gin.Context) {
	var page Page
	err := ctx.Bind(&page)
	if err != nil {
		return
	}
	uc := ctx.MustGet("user").(ijwt.UserClaims)
	arts, err := h.svc.GetByAuthor(ctx, uc.Uid, page.offset, page.limit)
	if err != nil {
		ctx.JSON(http.StatusOK, Result{
			Msg:  "系统错误",
			Code: 5,
		})
		h.l.Error("查找文章列表失败", logger2.Error(err),
			logger2.Int("pageOffset", page.offset),
			logger2.Int("pageLimit", page.limit),
			logger2.Int64("uid", uc.Uid))
		return
	}
	ctx.JSON(http.StatusOK, Result{
		//这表示把arts切片转换成ArticleVo，并且提供了转换方法
		Data: slice.Map[domain.Article, ArticleVo](arts, func(idx int, src domain.Article) ArticleVo {
			return ArticleVo{
				Id:       src.Id,
				Title:    src.Title,
				Abstract: src.Abstract(),
				AuthorId: src.Author.Id,
				Status:   src.Status.ToUint8(),
				Ctime:    src.Ctime.Format(time.DateTime),
				Utime:    src.Utime.Format(time.DateTime),
			}

		}),
	})
}

func (h *ArticleHandler) PubDetail(ctx *gin.Context) {
	idstr := ctx.Param("id")
	id, err := strconv.ParseInt(idstr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusOK, Result{
			Code: 4,
			Msg:  "id参数错误",
		})
		h.l.Warn("查询文章失败，id格式不对",
			logger2.String("id", idstr),
			logger2.Error(err))
		return
	}
	var (
		eg   errgroup.Group //常用来管理多条个并发执行
		art  domain.Article
		intr *intrv1.GetResponse
	)
	uc := ctx.MustGet("user").(ijwt.UserClaims)
	eg.Go(func() error {
		var er error
		art, er = h.svc.GetPubById(ctx, id, uc.Uid)
		return er
	})
	//异步中尽量少一些操作

	eg.Go(func() error {
		var er error
		intr, er = h.intrSvc.Get(ctx, &intrv1.GetRequest{
			Biz:   h.biz,
			BizId: id,
			Uid:   uc.Uid,
		})
		//查询该文章的交互数据，读者是要关心这些数据，把这些数据加入到前端交互中去
		if er != nil {
			return er
		}
		return err
	})
	//并发中两个进程并没有数据冲突，使用并发可以提高性能
	err = eg.Wait()
	if err != nil {
		ctx.JSON(http.StatusOK, Result{
			Msg:  "系统错误",
			Code: 5,
		})
		h.l.Error("查询文章失败，系统错误",
			logger2.Int64("artid", id),
			logger2.Int64("uid", uc.Uid),
			logger2.Error(err))
		return
	}
	//若进到这里，说明查询文章成功，所以阅读数需要加1
	//接入kafka后，以下代码都不需要了，前面会生成生产者消息
	//go func() {
	//	ctx, cancel := context.WithTimeout(context.Background(), time.Second) //执行这个异步函数，超时时间为1s
	//	defer cancel()
	//	er := h.intrSvc.IncrReadCnt(ctx, h.biz, id)
	//	if er != nil {
	//		h.l.Error("阅读数+1失败",
	//			logger.Int64("artid", id),
	//			logger.Int64("uid", uc.Uid),
	//			logger.Error(err))
	//	}
	//}()
	ctx.JSON(http.StatusOK, Result{
		Data: ArticleVo{
			Id:         art.Id,
			Title:      art.Title,
			Abstract:   art.Abstract(),
			Content:    art.Content,
			AuthorId:   art.Author.Id,
			AuthorName: art.Author.Name,
			Status:     art.Status.ToUint8(),
			Ctime:      art.Ctime.Format(time.DateTime),
			Utime:      art.Utime.Format(time.DateTime),
			ReadCnt:    intr.Intr.ReadCnt,
			LikeCnt:    intr.Intr.LikeCnt,
			CollectCnt: intr.Intr.CollectCnt,
			Liked:      intr.Intr.Liked,
			Collected:  intr.Intr.Collected,
		},
	})

}

func (h *ArticleHandler) Like(c *gin.Context) {
	type Req struct {
		Id   int64 `json:"id"`
		Like bool  `json:"like"`
	}
	var req Req
	err := c.Bind(&req)
	if err != nil {
		return
	}
	uc := c.MustGet("user").(ijwt.UserClaims)
	if req.Like {
		_, err = h.intrSvc.Like(c, &intrv1.LikeRequest{
			Biz:   h.biz,
			BizId: req.Id,
			Uid:   uc.Uid,
		})
	} else { //取消点赞
		_, err = h.intrSvc.CancelLike(c, &intrv1.CancelLikeRequest{
			Biz:   h.biz,
			BizId: req.Id,
			Uid:   uc.Uid,
		})
	}
	if err != nil {
		c.JSON(http.StatusOK, Result{
			Code: 5,
			Msg:  "点赞/取消点赞失败",
		})
		h.l.Error("点赞/取消点赞失败",
			logger2.Int64("artid", req.Id),
			logger2.Int64("uid", uc.Uid),
			logger2.Error(err))
	}
	c.JSON(http.StatusOK, Result{Msg: "ok"})
}

// 收藏和点赞都没有做到限制重复的功能，应该调用liked函数判断用户是否重复点赞
func (h *ArticleHandler) Collect(ctx *gin.Context) {
	type Req struct {
		Id  int64 `json:"id"`
		Cid int64 `json:"cid"` //收藏夹id？
	}
	var req Req
	err := ctx.Bind(&req)
	if err != nil {
		return
	}
	uc := ctx.MustGet("user").(ijwt.UserClaims)
	_, err = h.intrSvc.Collect(ctx, &intrv1.CollectRequest{
		Biz:   h.biz,
		BizId: req.Id,
		Uid:   uc.Uid,
		Cid:   req.Cid,
	})
	if err != nil {
		ctx.JSON(http.StatusOK, Result{
			Code: 5,
			Msg:  "收藏失败",
		})
		return
	}
	ctx.JSON(http.StatusOK, Result{Msg: "ok"})
}

func (h *ArticleHandler) Like100(ctx *gin.Context) {
	articles, err := h.svc.Like100(ctx, "article")
	if err != nil {
		ctx.JSON(http.StatusOK, Result{
			Code: 5,
			Msg:  "系统错误",
		})
		return
	}
	//转成切片后返回
	ctx.JSON(http.StatusOK, Result{
		Data: slice.Map[domain.Article, ArticleVo](articles, func(idx int, src domain.Article) ArticleVo {
			return ArticleVo{
				Id:       src.Id,
				Title:    src.Title,
				Abstract: src.Abstract(),
				AuthorId: src.Author.Id,
				Status:   src.Status.ToUint8(),
				Ctime:    src.Ctime.Format(time.DateTime),
			}
		}),
	})
}

type Page struct {
	offset int
	limit  int
}
