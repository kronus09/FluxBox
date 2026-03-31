package models

// TVConfig 是 影视Box 配置的顶层结构
type TVConfig struct {
	Spider    string   `json:"spider"`
	Wallpaper string   `json:"wallpaper"`
	Logo      string   `json:"logo"`
	Sites     []Site   `json:"sites"`
	Lives     []Live   `json:"lives,omitempty"`
	Ads       []string `json:"ads,omitempty"`
}

// Site 对应配置中的每一个影视源
type Site struct {
	Key         string      `json:"key"`
	Name        string      `json:"name"`
	Type        int         `json:"type"`
	Api         string      `json:"api"`
	Searchable  int         `json:"searchable,omitempty"`
	QuickSearch int         `json:"quickSearch,omitempty"`
	Filterable  int         `json:"filterable,omitempty"`
	Ext         interface{} `json:"ext,omitempty"`
	Jar         string      `json:"jar,omitempty"`
	Speed       int         `json:"-"` // 响应时间(毫秒)，不输出到JSON
}

// Live 对应电视直播配置
type Live struct {
	Name       string `json:"name"`
	Type       int    `json:"type"`
	Url        string `json:"url"`
	PlayerType int    `json:"playerType,omitempty"`
	UA         string `json:"ua,omitempty"`
	Group      string `json:"group,omitempty"`
}

// SourceItem 是 FluxBox 管理后台专用的模型
// 用于在 data/sources.json 中存储用户录入的原始源地址及抓取状态
type SourceItem struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	URL          string `json:"url"`
	Enabled      bool   `json:"enabled"`
	LastStatus   string `json:"last_status"`
	LastError    string `json:"last_error"`
	ResponseTime int    `json:"response_time"`
}
