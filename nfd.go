package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const lanzouMobileUA = "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Mobile/15E148 Safari/604.1"

// httpClient 带超时的公共客户端
var httpClient = &http.Client{Timeout: 30 * time.Second}

type Parser interface {
	// GetFileList 用于获取目录或单文件信息
	GetFileList(shareURL, pwd string) ([]FileItem, error)
	// GetDirectLink 根据文件 ID 获取真实直链（部分网盘的 CDN 还要求带回 cookie 头）
	GetDirectLink(fileID string) (directURL string, cookie string, err error)
}

type NfdParser struct {
	APIBase string
}

type FileItem struct {
	FileName  string `json:"fileName"`
	FileID    string `json:"fileId"`
	SizeStr   string `json:"sizeStr"`
	FileType  string `json:"fileType"`  // "folder" / "file"，NFD 目录接口自带
	ParserURL string `json:"parserUrl"` // NFD 自带的下一步地址（文件夹=子目录列表，文件=下载跳转）
	SubRef    string `json:"-"`         // 自建解析器的导航引用（夸克：qkdir:pwdID|stoken|fid）
}

type nfdListResponse struct {
	Code int        `json:"code"`
	Msg  string     `json:"msg"`
	Data []FileItem `json:"data"`
}

// 补全了单文件响应的结构，获取真实文件名
type nfdSingleResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		DirectLink string `json:"directLink"`
		FileInfo   struct {
			FileName string `json:"fileName"`
		} `json:"fileInfo"`
	} `json:"data"`
}

// isLanzouDirLink 判断是否蓝奏云目录分享链接
// 目录分享主流格式是 /s/xxx，老格式是 /b/xxx，都认
func isLanzouDirLink(shareURL string) bool {
	if !strings.Contains(shareURL, "lanzou") {
		return false
	}
	u, err := url.Parse(shareURL)
	if err != nil {
		return false
	}
	p := strings.TrimRight(u.Path, "/")
	return strings.HasPrefix(p, "/s/") || strings.HasPrefix(p, "/b/")
}

// is123PanLink 判断是否 123 云盘链接（官方域名一直在轮换，见过的都记上）
func is123PanLink(shareURL string) bool {
	for _, d := range []string{"123pan.com", "123pan.cn", "123865.com", "123684.com", "123912.com"} {
		if strings.Contains(shareURL, d) {
			return true
		}
	}
	return false
}

// nfdPanType 按域名返回 NFD 的网盘类型标识（对应 /json/:type/:key 路由）
func nfdPanType(shareURL string) string {
	switch {
	case is123PanLink(shareURL):
		return "ye"
	case strings.Contains(shareURL, "pan.quark.cn"):
		return "qk"
	case strings.Contains(shareURL, "drive.uc.cn") || strings.Contains(shareURL, "fast.uc.cn"):
		return "uc"
	case strings.Contains(shareURL, "139.com"):
		return "ec" // 移动云云空间
	}
	return ""
}

// panReferer 按网盘域名返回 aria2 下载用的 Referer；认不出就返回空串
func panReferer(shareURL string) string {
	switch nfdPanType(shareURL) {
	case "ye":
		return "https://www.123pan.com/"
	case "qk":
		return "https://pan.quark.cn/"
	case "uc":
		return "https://fast.uc.cn/"
	case "ec":
		return "https://caiyun.139.com/"
	}
	if strings.Contains(shareURL, "lanzou") {
		return "https://www.lanzou.com/"
	}
	return ""
}

// getDirList 走 NFD 的目录解析接口拿文件列表
func (n *NfdParser) getDirList(shareURL, pwd string) ([]FileItem, error) {
	apiURL := fmt.Sprintf("%s/v2/getFileList?url=%s", n.APIBase, url.QueryEscape(shareURL))
	if pwd != "" {
		// 只用 pwd 参数——NFD 前端契约就是它；
		// 多塞 password 会被 NFD 合并进分享 URL，把网盘类型识别搞坏
		apiURL += "&pwd=" + url.QueryEscape(pwd)
	}

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("主人请求列表失败了喵~: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var listResp nfdListResponse
	json.Unmarshal(body, &listResp)

	if listResp.Code != 200 {
		return nil, fmt.Errorf("%s", listResp.Msg)
	}
	return listResp.Data, nil
}

// firstMatch 正则抓第一个分组，抓不到就返回空串
func firstMatch(s, pattern string) string {
	m := regexp.MustCompile(pattern).FindStringSubmatch(s)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

// httpGetString 带移动端 UA 的 GET
func httpGetString(urlStr string) (string, error) {
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", lanzouMobileUA)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// getLanzouSDirList 自己解析蓝奏云 /s/ 新式目录（含老 /b/ 页面同源结构）：
// 1. 移动端 UA 拿目录页，从内联 JS 里抠出 fid/uid/puid/t/k
// 2. 分页 POST filemoreajax.php 拿文件列表（每页 50 条）
// 3. 每个文件页是 https://{host}/{id}，FileID 直接存完整 URL，
//    让现有的 GetDirectLink 走 NFD 单文件接口
func (n *NfdParser) getLanzouSDirList(shareURL, pwd string) ([]FileItem, error) {
	u, err := url.Parse(shareURL)
	if err != nil {
		return nil, fmt.Errorf("链接格式不对喵: %v", err)
	}
	base := u.Scheme + "://" + u.Host

	// 页面偶发会拿到 JS 挑战空壳（没有 fid/t/k），多重试几次
	var pageBody string
	var fid, uid, puid, tokenT, tokenK string
	for attempt := 1; attempt <= 3; attempt++ {
		var fetchErr error
		pageBody, fetchErr = httpGetString(base + u.Path)
		if fetchErr == nil {
			fid = firstMatch(pageBody, `'fid':(\d+)`)
			uid = firstMatch(pageBody, `'uid':'(\d+)'`)
			puid = firstMatch(pageBody, `'puid':'([^']+)'`)
			// t/k 是蓝奏云混淆过的 JS 变量（iblk0z/_hjw2s），名字会换，
			// 所以先按名字抓，抓不到再按"任意 var 赋 9 位数字/32 位十六进制"泛化兜底
			tokenT = firstMatch(pageBody, `iblk0z\s*=\s*'(\d{9,})'`)
			if tokenT == "" {
				tokenT = firstMatch(pageBody, `var\s+\w+\s*=\s*'(\d{9,})'`)
			}
			tokenK = firstMatch(pageBody, `_hjw2s\s*=\s*'([a-f0-9]{32})'`)
			if tokenK == "" {
				tokenK = firstMatch(pageBody, `var\s+\w+\s*=\s*'([a-f0-9]{32})'`)
			}
			if fid != "" && uid != "" && tokenT != "" && tokenK != "" {
				break
			}
		}
		if attempt == 3 {
			return nil, fmt.Errorf("目录页拿不到 fid/t/k 参数（蓝奏云可能在挑刺机器人，或页面结构又变了）喵: %v", fetchErr)
		}
		time.Sleep(400 * time.Millisecond)
	}

	var items []FileItem
	for pg := 1; pg <= 20; pg++ { // 一页 50 条，最多翻 20 页防失控
		form := url.Values{}
		form.Set("lx", "2")
		form.Set("fid", fid)
		form.Set("uid", uid)
		form.Set("puid", puid)
		form.Set("pg", strconv.Itoa(pg))
		form.Set("rep", "0")
		form.Set("t", tokenT)
		form.Set("k", tokenK)
		form.Set("up", "1")
		if pwd != "" {
			form.Set("pwd", pwd)
		}

		req, reqErr := http.NewRequest("POST", base+"/filemoreajax.php?file="+fid, strings.NewReader(form.Encode()))
		if reqErr != nil {
			return nil, reqErr
		}
		req.Header.Set("User-Agent", lanzouMobileUA)
		req.Header.Set("Referer", shareURL)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("X-Requested-With", "XMLHttpRequest") // 模拟 jQuery ajax

		resp, doErr := httpClient.Do(req)
		if doErr != nil {
			return nil, fmt.Errorf("请求文件列表失败喵: %v", doErr)
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}

		// zt 有数字/字符串两种形态，Text 失败时可能是 0 不是数组，都放宽处理
		var ajaxResp struct {
			ZT   interface{}     `json:"zt"`
			Info string          `json:"info"`
			Text json.RawMessage `json:"text"`
		}
		if json.Unmarshal(body, &ajaxResp) != nil {
			return nil, fmt.Errorf("文件列表响应解析失败喵")
		}

		zt := strings.TrimSpace(fmt.Sprintf("%v", ajaxResp.ZT))
		if zt != "1" {
			// 蓝奏云对"翻到底"的回应不是固定的 zt=2（可能是 4/6/其他），
			// 官网 JS 的做法就是非 1 就当没有下一页，照抄喵
			if pg == 1 && zt == "2" {
				break // 空目录
			}
			if pg == 1 {
				return nil, fmt.Errorf("蓝奏云目录获取失败喵: %s", ajaxResp.Info)
			}
			break
		}

		var entries []lanzouFileEntry
		_ = json.Unmarshal(ajaxResp.Text, &entries)
		for _, e := range entries {
			if e.T == 1 || e.ID == "" || e.ID == "-1" {
				continue // 跳过推广和占位条目
			}
			items = append(items, FileItem{
				FileName: e.NameAll,
				FileID:   base + "/" + e.ID,
				SizeStr:  e.Size + " | " + e.Time,
			})
		}
		if len(entries) < 50 {
			break
		}
	}
	return items, nil
}

// lanzouFileEntry 蓝奏云 filemoreajax 返回的单个文件条目
type lanzouFileEntry struct {
	ID      string `json:"id"`      // 文件短码，文件页就是 https://{host}/{id}
	NameAll string `json:"name_all"` // 文件名
	Size    string `json:"size"`
	Time    string `json:"time"`
	T       int    `json:"t"` // 1=推广条目，跳过
}

// GetFileList 获取目录列表或单文件的信息
func (n *NfdParser) GetFileList(shareURL, pwd string) ([]FileItem, error) {
	// 蓝奏云目录
	if isLanzouDirLink(shareURL) {
		// 优先走本喵自己的移动端解析：
		// NFD 自带的目录解析器用桌面 UA，会被蓝奏云的 JS 挑战拦下拿到空壳页，
		// 对 /s/ 新式目录只会报"获取失败1"
		items, dirErr := n.getLanzouSDirList(shareURL, pwd)
		if dirErr == nil {
			return items, nil
		}
		// 兜底退回 NFD 自带目录接口（老式 /b/ 链接、带密码目录归它管）
		items2, nfdErr := n.getDirList(shareURL, pwd)
		if nfdErr == nil {
			return items2, nil
		}
		// 两条通道都倒下时，把真实死因都亮出来
		return nil, fmt.Errorf("%v（备用通道: %v）", dirErr, nfdErr)
	}

	// 123云盘：单文件和目录分享都是 /s/Key 同形链接，分不出来，
	// 先试目录接口，失败再回单文件接口
	// 注意：只有 123盘走目录接口！夸克/UC/移动云的目录接口这版 NFD 不支持，
	// 而且实测对它们调 /v2/getFileList 会把 JVM 悄悄干死（无报错无转储），千万别碰
	if is123PanLink(shareURL) {
		items, dirErr := n.getDirList(shareURL, pwd)
		if dirErr == nil {
			markPanType(items, "ye")
			return items, nil
		}
		single, singleErr := n.getSingleFile(shareURL, pwd)
		if singleErr == nil {
			return single, nil
		}
		return nil, fmt.Errorf("%v（目录接口: %v）", singleErr, dirErr)
	}

	// 夸克：自建解析器（官方 API 全程匿名，NFD 这版不支持夸克目录）
	if nfdPanType(shareURL) == "qk" {
		return quarkFileList(shareURL, pwd)
	}

	// UC网盘：同夸克，自建解析器（NFD 这版不支持 UC 目录）
	if nfdPanType(shareURL) == "uc" {
		return ucFileList(shareURL, pwd)
	}

	// 移动云：新门户 yun.139.com 是 SPA+签名网关，NFD 的 EcTool 只实现了
	// 已废弃的 ecpan.cn 老接口（实测必炸）。自建解析器待建，先给明白的提示
	if nfdPanType(shareURL) == "ec" {
		return nil, fmt.Errorf("移动云暂未支持喵：新门户接口需要专门逆向（施工中），先用官方客户端或换其他通道")
	}

	// 其他链接按单文件处理
	return n.getSingleFile(shareURL, pwd)
}

// markPanType 给 123 盘目录条目做整理：
// 文件夹保留 parserUrl 供"进入文件夹"；文件从 parserUrl 抠出 redirectUrl 令牌给 GetDirectLink
func markPanType(items []FileItem, prefix string) {
	blobRe := regexp.MustCompile(`/v2/redirectUrl/` + prefix + `/([^"&\s]+)`)
	for i := range items {
		it := &items[i]
		if it.FileType == "folder" {
			continue // 走目录导航，不需要直链
		}
		if m := blobRe.FindStringSubmatch(it.ParserURL); m != nil {
			it.FileID = prefix + ":" + m[1]
			continue
		}
		if !strings.HasPrefix(it.FileID, "http") && !strings.Contains(it.FileID, ":") {
			it.FileID = prefix + ":" + it.FileID // 兜底旧路径
		}
	}
}

// GetFileListVia 取子目录内容：qkdir:/ucdir: 前缀走自建导航，其余是 NFD 的列表 URL
func (n *NfdParser) GetFileListVia(apiURL string) ([]FileItem, error) {
	// 夸克自建导航：qkdir:pwdID|stoken|fid
	if strings.HasPrefix(apiURL, "qkdir:") {
		parts := strings.Split(strings.TrimPrefix(apiURL, "qkdir:"), "|")
		if len(parts) != 3 {
			return nil, fmt.Errorf("夸克目录导航标识不对喵")
		}
		entries, err := quarkListDir(parts[0], parts[1], parts[2])
		if err != nil {
			return nil, err
		}
		return quarkToItems(parts[0], parts[1], entries), nil
	}

	// UC 自建导航：ucdir:pwdID|stoken|fid
	if strings.HasPrefix(apiURL, "ucdir:") {
		parts := strings.Split(strings.TrimPrefix(apiURL, "ucdir:"), "|")
		if len(parts) != 3 {
			return nil, fmt.Errorf("UC 目录导航标识不对喵")
		}
		entries, err := ucListDir(parts[0], parts[1], parts[2])
		if err != nil {
			return nil, err
		}
		return ucToItems(parts[0], parts[1], entries), nil
	}

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("请求子目录失败喵: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var listResp nfdListResponse
	json.Unmarshal(body, &listResp)

	if listResp.Code != 200 {
		return nil, fmt.Errorf("%s", listResp.Msg)
	}
	if is123PanLink(apiURL) {
		markPanType(listResp.Data, "ye")
	}
	return listResp.Data, nil
}

// injectPwd 把提取码补进 parserUrl 尾部的 pwd= 里
func injectPwd(apiURL, pwd string) string {
	if pwd == "" {
		return apiURL
	}
	if strings.HasSuffix(apiURL, "pwd=") || strings.Contains(apiURL, "pwd=&") {
		return apiURL + url.QueryEscape(pwd)
	}
	if !strings.Contains(apiURL, "pwd=") {
		if strings.Contains(apiURL, "?") {
			return apiURL + "&pwd=" + url.QueryEscape(pwd)
		}
		return apiURL + "?pwd=" + url.QueryEscape(pwd)
	}
	return apiURL
}

// stripPwdQuery 把分享链接里的 ?pwd=xxx 查询串剥掉
// 提取码已单独提取传参，查询串留在 URL 里会让 NFD 的网盘类型识别翻车（误路由到"其他网盘"）
func stripPwdQuery(rawurl string) string {
	u, err := url.Parse(rawurl)
	if err != nil || u.RawQuery == "" || !strings.Contains(u.RawQuery, "pwd=") {
		return rawurl
	}
	u.RawQuery = ""
	return u.String()
}

// getSingleFile 走 NFD 单文件接口解析，包装成单元素列表返回
func (n *NfdParser) getSingleFile(shareURL, pwd string) ([]FileItem, error) {
	apiURL := fmt.Sprintf("%s/json/parser?url=%s", n.APIBase, url.QueryEscape(shareURL))
	if pwd != "" {
		// 只带 pwd 参数；password 会被 NFD 合并进 URL 破坏路由，别加
		apiURL += "&pwd=" + url.QueryEscape(pwd)
	}
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("主人~请求单文件失败喵: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var singleResp nfdSingleResponse
	json.Unmarshal(body, &singleResp)

	if singleResp.Code != 200 {
		// 兜底：lanzou 域名/格式经常翻新，万一漏判成单文件，
		// 后端会提示"该链接为目录分享"，那就自动转目录接口再试一次
		if strings.Contains(singleResp.Msg, "目录") {
			return n.getDirList(shareURL, pwd)
		}
		return nil, fmt.Errorf("%s", singleResp.Msg)
	}

	// 修复：将原本写死的 "单文件"，改成获取真实文件名
	realFileName := singleResp.Data.FileInfo.FileName
	if realFileName == "" {
		realFileName = "未知文件" // 兜底
	}

	// 如果是单文件，包装成只有一个元素的列表返回
	return []FileItem{{FileName: realFileName, FileID: shareURL, SizeStr: "未知"}}, nil
}

// GetDirectLink 获取直链（第二个返回值是部分网盘 CDN 要验的 Cookie 头，没有就空串）
// fileID 形态：完整 URL（单文件）、"ye:" 前缀（123盘目录内文件）、
// "qk:" 前缀（夸克目录内文件，自建解析器）、其他类型前缀、裸短码（蓝奏云目录内文件）
func (n *NfdParser) GetDirectLink(fileID string) (string, string, error) {
	var apiURL string
	switch {
	case strings.HasPrefix(fileID, "http"):
		// 单文件：原链接塞在 FileID 里
		apiURL = fmt.Sprintf("%s/json/parser?url=%s", n.APIBase, url.QueryEscape(fileID))
	case strings.HasPrefix(fileID, "ye:"):
		// 123云盘目录内文件：走 NFD 的下载跳转端点，本喵自己摘 302 的 Location 当直链
		hop := fmt.Sprintf("%s/v2/redirectUrl/ye/%s", n.APIBase, url.PathEscape(strings.TrimPrefix(fileID, "ye:")))
		link, err := resolveNfdRedirect(hop)
		return link, "", err
	case strings.HasPrefix(fileID, "qk:"):
		// 夸克目录内文件：自建解析器直连官方 API（直链要配 cookie 头）
		return quarkResolveDownload(fileID)
	case strings.HasPrefix(fileID, "uc:"):
		// UC 目录内文件：自建解析器直连官方 API（直链要配 cookie 头）
		return ucResolveDownload(fileID)
	case strings.Contains(fileID, ":"):
		// 其他网盘类型前缀（uc:/ec:...），走 NFD 通用路由 json/:type/:key
		idx := strings.Index(fileID, ":")
		apiURL = fmt.Sprintf("%s/json/%s/%s", n.APIBase, fileID[:idx], url.PathEscape(fileID[idx+1:]))
	default:
		// 蓝奏云目录里的文件
		apiURL = fmt.Sprintf("%s/json/lz/%s", n.APIBase, url.PathEscape(fileID))
	}

	resp, err := http.Get(apiURL)
	if err != nil {
		return "", "", fmt.Errorf("呜呜呜,请求直链失败喵: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var singleResp nfdSingleResponse
	json.Unmarshal(body, &singleResp)

	if singleResp.Code != 200 {
		return "", "", fmt.Errorf("%s", singleResp.Msg)
	}
	return singleResp.Data.DirectLink, "", nil
}

// resolveNfdRedirect 访问 NFD 的下载跳转端点：
// 302 → 摘 Location 当真实直链；JSON 报错 → 原样透传；既不跳转也没报错 → 当代理流地址直接给 aria2
func resolveNfdRedirect(nfdURL string) (string, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // 不自动跟随，本喵自己摘 Location
		},
	}
	resp, err := client.Get(nfdURL)
	if err != nil {
		return "", fmt.Errorf("请求下载跳转失败喵: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		loc := resp.Header.Get("Location")
		if loc == "" {
			return "", fmt.Errorf("跳转响应里没有 Location 喵")
		}
		return loc, nil
	}

	body, _ := io.ReadAll(resp.Body)
	var r struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if json.Unmarshal(body, &r) == nil && r.Msg != "" {
		return "", fmt.Errorf("%s", r.Msg)
	}
	return nfdURL, nil // NFD 在代理出流量，把这地址直接给 aria2
}
