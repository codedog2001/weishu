package intergration

import (
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"testing"
	"xiaoweishu/webook/internal/intergration/startup"
	"xiaoweishu/webook/internal/repository/dao"
	ijwt "xiaoweishu/webook/internal/web/jwt"
)

type ArticleHandlerSuite struct {
	suite.Suite
	db    *gorm.DB
	sever *gin.Engine
}

//func (s *ArticleHandlerSuite) TearDownTest() {
//	err := s.db.Exec("truncate table `articles`").Error
//	assert.NoError(s.T(), err)
//	err = s.db.Exec("truncate table `published_articles`").Error
//	assert.NoError(s.T(), err)
//}

func TestArticleHandler(t *testing.T) {
	suite.Run(t, &ArticleHandlerSuite{})
}

// 初始化以及模拟登录态
func (s *ArticleHandlerSuite) SetupSuite() {
	s.db = startup.InitDB()
	//到这里发现是需要一个handler，所以回去定义接口
	hdl := startup.InitArticleHandler(dao.NewArticleGORMDAO(s.db))
	server := gin.Default()
	//这里相当于是模拟了用户登录态，只用登录的状态才会有jwttoken。设置uid
	server.Use(func(ctx *gin.Context) {
		ctx.Set("user", ijwt.UserClaims{
			Uid: 123,
		})
	})
	hdl.RegisterRoutes(server)
	s.sever = server
}
func (s *ArticleHandlerSuite) Test_Article_Publish() {
	t := s.T()

	testCases := []struct {
		name       string
		before     func(t *testing.T)
		after      func(t *testing.T)
		req        Article
		wantCode   int
		wantResult Result[int64]
	}{
		{
			name: "新建帖子并发表",
			before: func(t *testing.T) {
				//因为是新建帖子，并没有什么前置工作需要做
			},
			after: func(t *testing.T) {
				//验证数据
				//先验证制作库的数据
				//关系型数据库会自己根据表名去对应的表中查找数据
				var art dao.Article
				s.db.Where("author_id=?", 123).First(&art)
				assert.Equal(t, "测试标题", art.Title)
				assert.Equal(t, "测试内容", art.Content)
				assert.Equal(t, int64(123), art.AuthorId)
				assert.True(t, art.Ctime > 0)
				assert.True(t, art.Utime > 0)
				//接着验证线上库的数据
				var publishedArt dao.PublishedArticle
				s.db.Where("author_id=?", 123).First(&publishedArt)
				assert.Equal(t, "测试标题", publishedArt.Title)
				assert.Equal(t, "测试内容", publishedArt.Content)
				assert.Equal(t, int64(123), publishedArt.AuthorId)
				assert.Equal(t, uint8(2), publishedArt.Status)
				assert.True(t, publishedArt.Ctime > 0)
				assert.True(t, publishedArt.Utime > 0)
			},
			req: Article{
				Title:   "测试标题",
				Content: "测试内容",
			},
			wantCode: http.StatusOK,
			wantResult: Result[int64]{
				Data: 1,
			},
		},
		{
			name: "更新帖子并新发表",
			before: func(t *testing.T) {
				//准备数据
				s.db.Create(&dao.Article{
					Id:       2,
					Title:    "原标题",
					Content:  "原内容",
					AuthorId: 123,
					Status:   1,
					Ctime:    123,
					Utime:    123,
				})
			},
			after: func(t *testing.T) {
				//验证数据，不光要验证制作库，线上库一样是要验证
				var art dao.Article
				s.db.Where("id=?", 2).First(&art)
				assert.Equal(t, "新内容", art.Content)
				assert.Equal(t, "新标题", art.Title)
				assert.Equal(t, int64(123), art.AuthorId)
				assert.Equal(t, uint8(2), art.Status)
				assert.Equal(t, int64(123), art.Ctime)
				var pubArt dao.PublishedArticle
				s.db.Where("id=?", 2).First(&pubArt)
				assert.Equal(t, "新内容", pubArt.Content)
				assert.Equal(t, "新标题", pubArt.Title)
				assert.Equal(t, int64(123), pubArt.AuthorId)
				assert.Equal(t, uint8(2), pubArt.Status)
				assert.True(t, pubArt.Ctime > 0)
				assert.True(t, pubArt.Utime > art.Utime) //后修改的时间肯定比前面修改的时间要大
			},
			req: Article{
				Id:      2,
				Title:   "新标题",
				Content: "新内容",
			},
			wantCode: 200,
			wantResult: Result[int64]{
				Data: 2,
			},
		},
		{name: "更新帖子并且重新发表",
			before: func(t *testing.T) {
				s.db.Create(&dao.Article{
					Id:       3,
					Title:    "我的标题",
					Content:  "我的内容",
					Ctime:    234,
					Status:   1,
					Utime:    234,
					AuthorId: 123,
				})
				s.db.Create(&dao.PublishedArticle{
					Id:       3,
					Title:    "我的标题",
					Content:  "我的内容",
					Ctime:    234,
					Status:   1,
					Utime:    234,
					AuthorId: 123,
				})
			},
			after: func(t *testing.T) {
				var art dao.Article
				s.db.Where("id=?", 3).First(&art)
				assert.Equal(t, "我的标题", art.Title)
				assert.Equal(t, "我的内容", art.Content)
				assert.Equal(t, int64(123), art.AuthorId)
				assert.Equal(t, uint8(2), art.Status)
				assert.Equal(t, int64(234), art.Ctime)
				assert.True(t, art.Utime > 234)
				var pubArt dao.PublishedArticle
				s.db.Where("id=?", 3).First(&pubArt)
				assert.Equal(t, "我的标题", pubArt.Title)
				assert.Equal(t, "我的内容", pubArt.Content)
				assert.Equal(t, int64(123), pubArt.AuthorId)
				//同步到线上库的时候，创建时间不会变，但是更新时间会变
				assert.Equal(t, uint8(2), pubArt.Status)
				assert.True(t, pubArt.Utime > 234)
			},
			req: Article{
				Id:      3,
				Content: "我的内容",
				Title:   "我的标题",
			},
			wantCode: 200,
			wantResult: Result[int64]{
				Data: 3,
			},
		},
		{
			name: "更新别人的帖子，且发表失败",
			before: func(t *testing.T) {
				s.db.Create(&dao.Article{
					Id:       4,
					Title:    "别人的标题",
					Content:  "别人的内容",
					AuthorId: 456,
					Status:   1,
					Ctime:    234,
					Utime:    234,
				})
				s.db.Create(&dao.PublishedArticle{
					Id:       4,
					Title:    "别人的标题",
					Content:  "别人的内容",
					AuthorId: 456,
					Status:   1,
					Ctime:    234,
					Utime:    234,
				})
			},
			after: func(t *testing.T) {
				//更新必然失败，所以数据没有发生变化
				var art dao.Article
				s.db.Where("id=?", 4).First(&art)
				assert.Equal(t, "别人的标题", art.Title)
				assert.Equal(t, "别人的内容", art.Content)
				assert.Equal(t, int64(456), art.AuthorId)
				assert.Equal(t, uint8(1), art.Status)
				assert.Equal(t, int64(234), art.Ctime)
				assert.Equal(t, int64(234), art.Utime)
				var pubArt dao.PublishedArticle
				s.db.Where("id=?", 4).First(&pubArt)
				assert.Equal(t, "别人的标题", art.Title)
				assert.Equal(t, "别人的内容", art.Content)
				assert.Equal(t, int64(456), art.AuthorId)
				assert.Equal(t, uint8(1), art.Status)
				assert.Equal(t, int64(234), art.Ctime)
				assert.Equal(t, int64(234), art.Utime)
			},
			req: Article{
				Id:      4,
				Title:   "我的标题",
				Content: "我的内容",
			},
			wantCode: http.StatusOK,
			wantResult: Result[int64]{
				Code: 5,
				Msg:  "系统错误",
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.before(t)
			data, err := json.Marshal(tc.req)
			//断言这一步是不会出错的
			assert.NoError(t, err)
			req, err := http.NewRequest(http.MethodPost, "/articles/publish", bytes.NewReader(data))
			assert.NoError(t, err)
			//模拟请求
			req.Header.Set("Content-Type", "application/json") //设置文件类之后，bind方法就可以直接绑定
			recorder := httptest.NewRecorder()                 //用recorder来记录结果
			s.sever.ServeHTTP(recorder, req)
			code := recorder.Code
			assert.Equal(t, tc.wantCode, code)
			if code != http.StatusOK {
				return
			}
			var result Result[int64]
			err = json.Unmarshal(recorder.Body.Bytes(), &result)
			assert.NoError(t, err)
			assert.Equal(t, tc.wantResult, result)
			tc.after(t)
		})
	}
}
func (s *ArticleHandlerSuite) TestEdit() {
	t := s.T()
	testCases := []struct {
		name   string
		before func(t *testing.T)
		after  func(t *testing.T)
		//前端传过来的数据，在这里作为测试用列
		req      Article
		wantCode int
		wantRes  Result[int64]
	}{
		{
			name: "新建帖子",
			before: func(t *testing.T) {
				//新建帖子没有前置工作
			},
			after: func(t *testing.T) {
				//验证数据
				var art dao.Article
				s.db.Where("id=?", 1).First(&art)
				assert.Equal(t, "我的标题", art.Title)
				assert.Equal(t, "我的内容", art.Content)
				assert.Equal(t, int64(123), art.AuthorId)
				assert.Equal(t, uint8(1), art.Status)
				assert.True(t, art.Utime > 0)
				assert.True(t, art.Ctime > 0)
			},
			req: Article{
				Title:   "我的标题",
				Content: "我的内容",
			},
			wantCode: http.StatusOK,
			wantRes: Result[int64]{
				Data: 1,
				//写代码的时候，成功之后data放的是文章id
				//所以这里希望id是1
			},
		},
		{
			name: "修改帖子",
			before: func(t *testing.T) {
				s.db.Create(&dao.Article{
					Id:       2,
					Content:  "旧内容",
					Title:    "旧标题",
					AuthorId: 123,
					//设置未发表的帖子更容易进行测试
					//如果设置成发表，严格来说还需要插入线上表，但是后续验证数据又不用到线上表，所以多此一举
					//直接设置成未发表方便测试
					Status: 1,
					Ctime:  234,
					Utime:  234,
				})
			},
			after: func(t *testing.T) {
				var art dao.Article
				s.db.Where("id=?", 2).First(&art)
				assert.Equal(t, "新标题", art.Title)
				assert.Equal(t, "新内容", art.Content)
				assert.Equal(t, int64(123), art.AuthorId)
				assert.Equal(t, uint8(1), art.Status)
				assert.True(t, art.Utime > 234)
			},
			req: Article{
				Id:      2,
				Title:   "新标题",
				Content: "新内容",
			},
			wantCode: http.StatusOK,
			wantRes: Result[int64]{
				Data: 2,
			},
		},
		{
			name: "修改别人的帖子且失败",
			before: func(t *testing.T) {
				s.db.Create(&dao.Article{
					Id:       3,
					Content:  "旧内容",
					Title:    "旧标题",
					AuthorId: 456,
					Status:   1,
					Ctime:    234,
					Utime:    234,
				})
			},
			after: func(t *testing.T) {
				//验证数据,这里肯定是修改失败了，所以所有数据都相同
				var art dao.Article
				s.db.Where("id=?", 3).First(&art)
				assert.Equal(t, dao.Article{
					Id:       3,
					Content:  "旧内容",
					Title:    "旧标题",
					AuthorId: 456,
					Status:   1,
					Ctime:    234,
					Utime:    234,
				}, art)
			},
			req: Article{
				Id:      3,
				Content: "新内容",
				Title:   "新标题",
			},
			wantCode: http.StatusOK,
			wantRes: Result[int64]{
				Code: 5,
				Msg:  "系统错误",
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.before(t)
			defer tc.after(t)
			//把请求系列化成json格式，然后解释了传的是json格式，意思是我给你传的json格式文件，你用json格式来进行解析
			reqBody, err := json.Marshal(tc.req)
			assert.NoError(t, err) //判断这一步不会出错
			req, err := http.NewRequest(http.MethodPost, "/articles/edit", bytes.NewReader(reqBody))
			//该函数用于设置HTTP请求的头部信息中的"Content-Type"字段，
			//其值为"application/json"。这表明请求体中的数据格式为JSON格式。
			req.Header.Set("Content-Type", "application/json")
			assert.NoError(t, err)
			record := httptest.NewRecorder()
			//执行和模拟http请求
			s.sever.ServeHTTP(record, req)
			assert.Equal(t, tc.wantCode, record.Code)
			if tc.wantCode != http.StatusOK {
				return
			}
			var result Result[int64]
			err = json.Unmarshal(record.Body.Bytes(), &result)
			assert.NoError(t, err)
			assert.Equal(t, tc.wantRes, result)
		})
	}
}

type Article struct {
	Id      int64  `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}
type Result[T any] struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data T      `json:"data"`
}
