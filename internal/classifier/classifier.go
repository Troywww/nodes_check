package classifier

import (
	"net"
	"strings"

	"nodes_check/internal/parser"
)

const (
	CategoryHongKong    = "香港"
	CategoryAsia        = "亚洲"
	CategoryEurope      = "欧洲"
	CategoryAmerica     = "美洲"
	CategoryOtherRegion = "其他区域"
	CategoryMobile      = "移动"
	CategoryUnicom      = "联通"
	CategoryTelecom     = "电信"
	CategoryOfficial    = "官方优选"
	CategoryTypeRegion  = "大区"
	CategoryTypeCarrier = "运营商"
)

type Node struct {
	parser.Node
	Category     string
	CategoryType string
	SubRegion    string
	IsCloudflare bool
}

type rule struct {
	Keywords  []string
	Category  string
	Type      string
	SubRegion string
}

var carrierRules = []rule{
	{Keywords: []string{"移动", "cmcc", "china mobile"}, Category: CategoryMobile, Type: CategoryTypeCarrier},
	{Keywords: []string{"联通", "unicom", "china unicom"}, Category: CategoryUnicom, Type: CategoryTypeCarrier},
	{Keywords: []string{"电信", "telecom", "china telecom"}, Category: CategoryTelecom, Type: CategoryTypeCarrier},
}

var regionRules = []rule{
	{Keywords: []string{"香港", "hong kong", " hkg ", " hk ", "hk-", "-hk"}, Category: CategoryHongKong, Type: CategoryTypeRegion, SubRegion: "HK"},
	{Keywords: []string{"日本", "japan", "tokyo", "osaka", " jp ", "jp-", "-jp"}, Category: CategoryAsia, Type: CategoryTypeRegion, SubRegion: "JP"},
	{Keywords: []string{"新加坡", "singapore", " sg ", "sg-", "-sg"}, Category: CategoryAsia, Type: CategoryTypeRegion, SubRegion: "SG"},
	{Keywords: []string{"韩国", "korea", "seoul", " kr ", "kr-", "-kr"}, Category: CategoryAsia, Type: CategoryTypeRegion, SubRegion: "KR"},
	{Keywords: []string{"台湾", "taiwan", "台北", " tw ", "tw-", "-tw", "taipei"}, Category: CategoryAsia, Type: CategoryTypeRegion, SubRegion: "TW"},
	{Keywords: []string{"亚洲", "asia"}, Category: CategoryAsia, Type: CategoryTypeRegion},
	{Keywords: []string{"德国", "germany", "frankfurt", " de ", "de-", "-de"}, Category: CategoryEurope, Type: CategoryTypeRegion, SubRegion: "DE"},
	{Keywords: []string{"荷兰", "netherlands", "amsterdam", " nl ", "nl-", "-nl"}, Category: CategoryEurope, Type: CategoryTypeRegion, SubRegion: "NL"},
	{Keywords: []string{"法国", "france", "paris", " fr ", "fr-", "-fr"}, Category: CategoryEurope, Type: CategoryTypeRegion, SubRegion: "FR"},
	{Keywords: []string{"英国", "great britain", "united kingdom", "london", " gb ", " gb-", "-gb", " uk ", "uk-", "-uk"}, Category: CategoryEurope, Type: CategoryTypeRegion, SubRegion: "GB"},
	{Keywords: []string{"拉脱维亚", "latvia", " lv ", "lv-", "-lv"}, Category: CategoryEurope, Type: CategoryTypeRegion, SubRegion: "LV"},
	{Keywords: []string{"瑞典", "sweden", " se ", "se-", "-se"}, Category: CategoryEurope, Type: CategoryTypeRegion, SubRegion: "SE"},
	{Keywords: []string{"芬兰", "finland", " fi ", "fi-", "-fi"}, Category: CategoryEurope, Type: CategoryTypeRegion, SubRegion: "FI"},
	{Keywords: []string{"挪威", "norway", " no ", "no-", "-no"}, Category: CategoryEurope, Type: CategoryTypeRegion, SubRegion: "NO"},
	{Keywords: []string{"丹麦", "denmark", " dk ", "dk-", "-dk"}, Category: CategoryEurope, Type: CategoryTypeRegion, SubRegion: "DK"},
	{Keywords: []string{"波兰", "poland", " pl ", "pl-", "-pl"}, Category: CategoryEurope, Type: CategoryTypeRegion, SubRegion: "PL"},
	{Keywords: []string{"意大利", "italy", " it ", "it-", "-it"}, Category: CategoryEurope, Type: CategoryTypeRegion, SubRegion: "IT"},
	{Keywords: []string{"西班牙", "spain", " es ", "es-", "-es"}, Category: CategoryEurope, Type: CategoryTypeRegion, SubRegion: "ES"},
	{Keywords: []string{"瑞士", "switzerland", " ch ", "ch-", "-ch"}, Category: CategoryEurope, Type: CategoryTypeRegion, SubRegion: "CH"},
	{Keywords: []string{"奥地利", "austria", " at ", "at-", "-at"}, Category: CategoryEurope, Type: CategoryTypeRegion, SubRegion: "AT"},
	{Keywords: []string{"比利时", "belgium", " be ", "be-", "-be"}, Category: CategoryEurope, Type: CategoryTypeRegion, SubRegion: "BE"},
	{Keywords: []string{"爱尔兰", "ireland", " ie ", "ie-", "-ie"}, Category: CategoryEurope, Type: CategoryTypeRegion, SubRegion: "IE"},
	{Keywords: []string{"捷克", "czech", " cz ", "cz-", "-cz"}, Category: CategoryEurope, Type: CategoryTypeRegion, SubRegion: "CZ"},
	{Keywords: []string{"欧洲", "europe"}, Category: CategoryEurope, Type: CategoryTypeRegion},
	{Keywords: []string{"美国", "united states", "usa", "san jose", "los angeles", "new york", " us ", "us-", "-us"}, Category: CategoryAmerica, Type: CategoryTypeRegion, SubRegion: "US"},
	{Keywords: []string{"加拿大", "canada", "toronto", "vancouver", " ca ", "ca-", "-ca"}, Category: CategoryAmerica, Type: CategoryTypeRegion, SubRegion: "CA"},
	{Keywords: []string{"墨西哥", "mexico", " mx ", "mx-", "-mx"}, Category: CategoryAmerica, Type: CategoryTypeRegion, SubRegion: "MX"},
	{Keywords: []string{"巴西", "brazil", " br ", "br-", "-br"}, Category: CategoryAmerica, Type: CategoryTypeRegion, SubRegion: "BR"},
	{Keywords: []string{"智利", "chile", " cl ", "cl-", "-cl"}, Category: CategoryAmerica, Type: CategoryTypeRegion, SubRegion: "CL"},
	{Keywords: []string{"阿根廷", "argentina", " ar ", "ar-", "-ar"}, Category: CategoryAmerica, Type: CategoryTypeRegion, SubRegion: "AR"},
	{Keywords: []string{"美洲", "america"}, Category: CategoryAmerica, Type: CategoryTypeRegion},
}

var cloudflareCIDRs = mustParseCIDRs([]string{
	"173.245.48.0/20",
	"103.21.244.0/22",
	"103.22.200.0/22",
	"103.31.4.0/22",
	"141.101.64.0/18",
	"108.162.192.0/18",
	"190.93.240.0/20",
	"188.114.96.0/20",
	"197.234.240.0/22",
	"198.41.128.0/17",
	"162.158.0.0/15",
	"104.16.0.0/13",
	"104.24.0.0/14",
	"172.64.0.0/13",
	"131.0.72.0/22",
})

func ClassifyAll(nodes []parser.Node) []Node {
	classified := make([]Node, 0, len(nodes))
	for _, node := range nodes {
		classified = append(classified, Classify(node))
	}
	return classified
}

func Classify(node parser.Node) Node {
	normalized := " " + strings.ToLower(node.Name) + " "
	isCF := IsCloudflareIP(node.Host)

	for _, current := range carrierRules {
		for _, keyword := range current.Keywords {
			if strings.Contains(normalized, strings.ToLower(keyword)) {
				return Node{Node: node, Category: current.Category, CategoryType: current.Type, SubRegion: current.SubRegion, IsCloudflare: isCF}
			}
		}
	}

	if isCF {
		return Node{Node: node, Category: CategoryOfficial, CategoryType: CategoryTypeCarrier, IsCloudflare: true}
	}

	for _, current := range regionRules {
		for _, keyword := range current.Keywords {
			if strings.Contains(normalized, strings.ToLower(keyword)) {
				return Node{Node: node, Category: current.Category, CategoryType: current.Type, SubRegion: current.SubRegion, IsCloudflare: false}
			}
		}
	}

	return Node{Node: node, Category: CategoryOtherRegion, CategoryType: CategoryTypeRegion, IsCloudflare: false}
}

func IsDNSCategory(node Node) bool {
	switch node.Category {
	case CategoryMobile, CategoryUnicom, CategoryTelecom, CategoryOfficial:
		return true
	default:
		return false
	}
}

func IsCloudflareIP(host string) bool {
	ip := net.ParseIP(strings.TrimSpace(host))
	if ip == nil {
		return false
	}
	for _, network := range cloudflareCIDRs {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func mustParseCIDRs(values []string) []*net.IPNet {
	out := make([]*net.IPNet, 0, len(values))
	for _, value := range values {
		_, network, err := net.ParseCIDR(value)
		if err != nil {
			panic(err)
		}
		out = append(out, network)
	}
	return out
}
