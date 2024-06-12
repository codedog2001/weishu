package service

import (
	"context"
	"errors"
	"golang.org/x/crypto/bcrypt"
	"xiaoweishu/webook/internal/domain"
	"xiaoweishu/webook/internal/repository"
)

var ErrUserDuplicateEmail = repository.ErrUserDuplicateEmail
var ErrInvalidUserOrPassword = errors.New("账号/邮箱或密码不对")

type UserService interface {
	SignUp(ctx context.Context, u domain.User) error
	Login(ctx context.Context, email, password string) (domain.User, error)
	Profile(ctx context.Context, id int64) (domain.User, error)
	FindOrCrete(ctx context.Context, phone string) (domain.User, error)
	FindOrCrateByWechat(ctx context.Context, info domain.WechatInfo) (domain.User, error)
}

type userService struct {
	repo repository.UserRepository
}

func NewUserService(repo repository.UserRepository) UserService {
	return &userService{repo: repo}
	//因为userservice实现了UserService的所有方法
	//所以实际上是可以var userService UserService

}

// SignUp 不能用上一层的结构体数据，而是要用下一层传上来的数据进行操作
// service层的注册要做的事情,不需要传指针，传指针还需要进行判空
// 加密更应该放在sercive层面，这样后面的层也是加密的，更安全
func (svc *userService) SignUp(ctx context.Context, u domain.User) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hash) //返回的加密密码也是字节切片，要转换成字符串

	return svc.repo.Create(ctx, u)
	//再service层直接去调用下一层，service层面一般都不用去做什么具体操作，有错误则返回，再到上一层去处理错误
}
func (svc *userService) Login(ctx context.Context, email, password string) (domain.User, error) {
	//先找用户
	u, err := svc.repo.FindByEmail(ctx, email)
	//这是找不到邮箱的情况
	//注意比较错误的分支要先写在前面，不然的话就会走下面的err!=nil
	if errors.Is(err, repository.ErrUserNotFound) {
		return domain.User{}, ErrInvalidUserOrPassword
	}
	if err != nil {
		return domain.User{}, err
	}

	//通过邮箱找到了这个用户，说明数据库中是有这个人的，说明他已经注册过了，所以现在要开始进行比较密码
	err = bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	if err != nil {
		//后面要在这里进行打印日志
		return domain.User{}, ErrInvalidUserOrPassword
	}
	return u, nil

}
func (svc *userService) Profile(ctx context.Context, id int64) (domain.User, error) {
	u, err := svc.repo.FindById(ctx, id)
	if err != nil {
		return domain.User{}, err
	}
	return u, nil

}
func (svc *userService) FindOrCrete(ctx context.Context, phone string) (domain.User, error) {
	u, err := svc.repo.FindByPhone(ctx, phone)
	if !errors.Is(err, repository.ErrUserNotFound) {
		//err==nil or !=notfound 都会进来这个分支，然后返回
		//往下走就说明确实是not found ，那就为这个手机号创建这个id
		return u, err
	}
	//如果走到这个分支，那么就可以确定确实是没有这个分支，进行创建即可
	u = domain.User{Phone: phone} //到这里只能拿到用户手机号，
	err = svc.repo.Create(ctx, u)
	if err != nil {
		return u, err
	}
	//不能直接返回u,因为这个u是不完整的，只有phone数据，不能提供后面的数据所需
	return svc.repo.FindByPhone(ctx, phone)
}
func (svc *userService) FindOrCrateByWechat(ctx context.Context, info domain.WechatInfo) (domain.User, error) {
	u, err := svc.repo.FindByWechat(ctx, info.OpenId)
	if !errors.Is(err, repository.ErrUserNotFound) {
		//err==nil or !=notfound 都会进来这个分支，然后返回
		//往下走就说明确实是not found ，那就为这个手机号创建这个id
		return u, err
	}
	//如果走到这个分支，那么就可以确定确实是没有这个分支，进行创建即可
	u = domain.User{Phone: info.OpenId} //到这里只能拿到用户手机号，
	err = svc.repo.Create(ctx, u)
	if err != nil {
		return u, err
	}
	//不能直接返回u,因为这个u是不完整的，只有phone数据，不能提供后面的数据所需
	return svc.repo.FindByWechat(ctx, info.OpenId)
}

//
