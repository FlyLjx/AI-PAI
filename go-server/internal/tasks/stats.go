package tasks

import "errors"

var (
	ErrNoResultImage       = errors.New("只有成功生成的图片可以操作")
	ErrInvalidPublicStatus = errors.New("公开审核状态不正确")
	ErrForbiddenTask       = errors.New("不能操作其他用户的任务")
)

type Stats struct {
	Total       int `json:"total"`
	Queued      int `json:"queued"`
	Pending     int `json:"pending"`
	Processing  int `json:"processing"`
	Success     int `json:"success"`
	Failed      int `json:"failed"`
	Canceled    int `json:"canceled"`
	TotalImages int `json:"totalImages"`
}
