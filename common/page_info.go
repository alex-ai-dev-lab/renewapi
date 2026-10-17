package common

import (
	"math"
	"strconv"

	"github.com/gin-gonic/gin"
)

type PageInfo struct {
	Page     int `json:"page"`      // page num 页码
	PageSize int `json:"page_size"` // page size 页大小

	Total       int  `json:"total"` // 总条数，后设置
	TotalCapped bool `json:"total_capped,omitempty"`
	Items       any  `json:"items"` // 数据，后设置
}

func (p *PageInfo) GetStartIdx() int {
	return (p.Page - 1) * p.PageSize
}

func (p *PageInfo) GetEndIdx() int {
	return p.Page * p.PageSize
}

func (p *PageInfo) GetPageSize() int {
	return p.PageSize
}

func (p *PageInfo) GetPage() int {
	return p.Page
}

func (p *PageInfo) SetTotal(total int) {
	p.Total = total
}

func (p *PageInfo) SetItems(items any) {
	p.Items = items
}

func GetPageQuery(c *gin.Context) *PageInfo {
	pageInfo := &PageInfo{Page: 1, PageSize: min(max(ItemsPerPage, 1), 100)}
	if page, err := strconv.Atoi(c.Query("p")); err == nil && page > 0 {
		pageInfo.Page = page
	}
	for _, key := range []string{"page_size", "ps", "size"} {
		if raw, exists := c.GetQuery(key); exists {
			if size, err := strconv.Atoi(raw); err == nil && size > 0 {
				pageInfo.PageSize = min(size, 100)
			}
			break
		}
	}
	// Bound both start/end arithmetic even for extreme positive integers.
	if pageInfo.Page > math.MaxInt/pageInfo.PageSize {
		pageInfo.Page = math.MaxInt / pageInfo.PageSize
	}
	return pageInfo
}
