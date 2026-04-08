package models

// TVConfig 是 影视Box 配置的顶层结构
type TVConfig struct {
	Spider     string        `json:"spider,omitempty"`
	Wallpaper  string        `json:"wallpaper,omitempty"`
	Logo       string        `json:"logo,omitempty"`
	Sites      []Site        `json:"sites,omitempty"`
	Lives      []Live        `json:"lives,omitempty"`
	Ads        []string      `json:"ads,omitempty"`
	Urls       []VideoSource `json:"urls,omitempty"`
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
	
	HealthScore   int    `json:"health_score"`
	HealthStatus  string `json:"health_status"`
	SiteTotal     int    `json:"site_total"`
	SiteCrawler   int    `json:"site_crawler"`
	SiteCollector int    `json:"site_collector"`
	JarTotal      int    `json:"jar_total"`
	JarSuccess    int    `json:"jar_success"`
	JarFailed     int    `json:"jar_failed"`
	JarCached     int    `json:"jar_cached"`
	LastCheckTime string `json:"last_check_time"`
	
	// 本地化相关
	Localized   bool   `json:"localized"`    // 是否已本地化
	LocalStatus string `json:"local_status"` // success / failed / pending
	LocalTime   string `json:"local_time"`   // 本地化时间
	LocalError  string `json:"local_error"`  // 失败原因
}

// ScheduleConfig 计划任务配置
type ScheduleConfig struct {
	Enabled   bool   `json:"enabled"`   // 是否启用
	Frequency string `json:"frequency"` // daily / weekly
	Time      string `json:"time"`      // HH:mm 格式
	Days      []int  `json:"days"`      // 1=周一 ... 7=周日，weekly模式使用
	Mode      string `json:"mode"`      // fastest / all，聚合模式
	MaxSites  int    `json:"max_sites"` // 最快模式下最大站点数，默认120
}

// GlobalConfig 全局配置
type GlobalConfig struct {
	// 单仓聚合（保持现有字段）
	AggMode        string   `json:"agg_mode"`        // fastest / all
	MaxSites       int      `json:"max_sites"`       // 60-200
	FilterWords    []string `json:"filter_words"`    // 过滤关键词
	HomeMenuSource int      `json:"home_menu_source"` // 首页菜单来源，0表示自动选择

	// 多仓配置
	MultiIncludeWarning bool `json:"multi_include_warning"` // 包含警告源（60-80分）
	MultiPreferLocal    bool `json:"multi_prefer_local"`    // 优先使用本地化源

	// 健康检查 - 自动禁用
	AutoDisableUnhealthy bool `json:"auto_disable_unhealthy"` // 🔴 不健康（<60分）
	AutoDisableWarning   bool `json:"auto_disable_warning"`   // 🟡 警告（60-80分）
	AutoDisableFailed    bool `json:"auto_disable_failed"`    // ⚫ 失效（<30分）

	// 计划任务
	ScheduleEnabled bool `json:"schedule_enabled"` // 计划任务总开关

	// 聚合任务（单仓+多仓）
	AggSingleEnabled    bool   `json:"agg_single_enabled"`    // 单仓聚合开关
	AggMultiEnabled     bool   `json:"agg_multi_enabled"`     // 多仓生成开关
	AggScheduleFreq     string `json:"agg_schedule_freq"`     // daily / weekly
	AggScheduleTime     string `json:"agg_schedule_time"`     // HH:mm
	AggScheduleDays     []int  `json:"agg_schedule_days"`     // 周几

	// 健康检查任务
	HealthScheduleEnabled bool   `json:"health_schedule_enabled"` // 健康检查开关
	HealthScheduleFreq    string `json:"health_schedule_freq"`    // daily / weekly
	HealthScheduleTime    string `json:"health_schedule_time"`    // HH:mm
	HealthScheduleDays    []int  `json:"health_schedule_days"`    // 周几
}
