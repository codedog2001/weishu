package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"xiaoweishu/webook/internal/repository"
	"xiaoweishu/webook/internal/service/sms"
)

const codeTplid = "1877556"

var (
	ErrCodeVerifyTooManyTimes = repository.ErrCodeVerifyTooMany
	ErrCodeSendTooMany        = repository.ErrCodeSendTOOMany
)

//	CodeService type CodeService interface {
//		Send(ctx context.Context, biz, phone string) error
//		Verify(ctx context.Context,
//			biz, phone, inputCode string) (bool, error)
//	}
type CodeSerVice interface {
	Send(ctx context.Context, biz string, phone string) error
	Verify(ctx context.Context, biz, phone, inputCode string) (bool, error)
}
type codeService struct {
	repo   repository.CodeRepository
	smsSVC sms.Service
}

func NewCodeService(repo repository.CodeRepository, smsSVC sms.Service) CodeSerVice {
	return &codeService{
		repo:   repo,
		smsSVC: smsSVC,
	}
}

func (svc *codeService) Send(ctx context.Context, biz string, phone string) error {
	//biz是用于区别业务场景，比如登录和修改等不同的业务
	//生成一个验证码
	code := svc.generateCode()
	err := svc.repo.Store(ctx, biz, phone, code)
	if err != nil {
		//写入redis时失败
		return err
	}
	//塞进去redis
	err = svc.smsSVC.Send(ctx, codeTplid, []string{code}, phone)
	if err != nil {
		//这里说明写入redis成功，但是发送失败了
		//可以在struct结构定义一个可以重发的接口，直接调用
		return err
	}
	return err

	//发送出去
}
func (svc *codeService) Verify(ctx context.Context, biz, phone, inputCode string) (bool, error) {
	ok, err := svc.repo.Verify(ctx, biz, phone, inputCode)
	if errors.Is(err, repository.ErrCodeVerifyTooMany) {
		// 相当于，我们对外面屏蔽了验证次数过多的错误，我们就是告诉调用者，你这个不对
		return false, nil
	}
	return ok, err
}

func (svc *codeService) generateCode() string {
	num := rand.Intn(1000000)
	return fmt.Sprintf("%06d", num)
}
