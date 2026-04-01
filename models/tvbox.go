package models

// TVConfig 是 影视Box 配置的顶层结构
type TVConfig struct {
	Spider     string        `json:"spider"`
	Wallpaper  string        `json:"wallpaper"`
	Logo       string        `json:"logo"`
	Sites      []Site        `json:"sites"`
	Lives      []Live        `json:"lives,omitempty"`
	Ads        []string      `json:"ads,omitempty"`
	VideoList  []VideoSource `json:"videoList,omitempty"`
	Parses     []Parse       `json:"parses,omitempty"`
	Rules      []Rule        `json:"rules,omitempty"`
	Flags      []string      `json:"flags,omitempty"`
	Ijk        []Ijk         `json:"ijk,omitempty"`
	Doh        []Doh         `json:"doh,omitempty"`
}

// VideoSource 多仓配置中的子仓库
type VideoSource struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Type string `json:"type,omitempty"`
}

// Parse 解析器配置
type Parse struct {
	Name string      `json:"name"`
	Type interface{} `json:"type"`  // 可能是 int 或 string
	URL  string      `json:"url"`
	Ext  interface{} `json:"ext,omitempty"`
}

// Rule 解析规则
type Rule struct {
	Name   string   `json:"name"`
	Hosts  []string `json:"hosts"`
	Regex  []string `json:"regex,omitempty"`
	Script []string `json:"script,omitempty"`
}

// Ijk 播放器配置
type Ijk struct {
	Group   string        `json:"group"`
	Options []IjkOption   `json:"options"`
}

// IjkOption IJK播放器选项
type IjkOption struct {
	Category int    `json:"category"`
	Name     string `json:"name"`
	Value    string `json:"value"`
}

// Doh DNS over HTTPS 配置
type Doh struct {
	Name string   `json:"name"`
	URL  string   `json:"url"`
	Ips  []string `json:"ips,omitempty"`
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

// ScheduleConfig 计划任务配置
type ScheduleConfig struct {
	Enabled   bool   `json:"enabled"`   // 是否启用
	Frequency string `json:"frequency"` // daily / weekly
	Time      string `json:"time"`      // HH:mm 格式
	Days      []int  `json:"days"`      // 1=周一 ... 7=周日，weekly模式使用
	Mode      string `json:"mode"`      // fastest / all，聚合模式
}
