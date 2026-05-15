package service

import "errors"

var ErrEmailNotifyDisabled = errors.New("任务邮件通知功能暂未开启")
var ErrEmailNotifyEmailMissing = errors.New("当前账号未绑定邮箱，无法发送任务邮件通知")
