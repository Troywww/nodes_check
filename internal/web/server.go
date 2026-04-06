package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strings"

	"nodes_check/internal/app"
	"nodes_check/internal/config"
)

type Server struct {
	runner     *app.Runner
	configPath string
}

func New(runner *app.Runner, configPath string) *Server {
	return &Server{runner: runner, configPath: configPath}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", s.handleLogin)
	mux.HandleFunc("/logout", s.handleLogout)
	mux.HandleFunc("/", s.requireAuth(s.handleIndex))
	mux.HandleFunc("/api/state", s.requireAuth(s.handleState))
	mux.HandleFunc("/api/run", s.requireAuth(s.handleRun))
	mux.HandleFunc("/api/config", s.requireAuth(s.handleConfig))
	return mux
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.Load(s.configPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if r.Method == http.MethodGet {
		_, authed := s.authenticated(r, cfg.Web.AuthToken)
		if authed {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		_, _ = w.Write([]byte(loginHTML))
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	token := strings.TrimSpace(r.FormValue("token"))
	if token == "" || token != cfg.Web.AuthToken {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(loginHTML))
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "nodes_check_token", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: "nodes_check_token", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	cfgText, _ := os.ReadFile(s.configPath)
	cfg, err := config.Load(s.configPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	subText, _ := os.ReadFile(cfg.Subscription.File)
	data := struct {
		State         app.State
		ConfigPath    string
		Subscriptions string
		ConfigText    string
	}{
		State:         s.runner.State(),
		ConfigPath:    s.configPath,
		Subscriptions: string(subText),
		ConfigText:    string(cfgText),
	}
	tmpl := template.Must(template.New("index").Parse(indexHTML))
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(s.runner.State())
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.runner.RunAsync(); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte("started"))
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg, err := config.Load(s.configPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		cfgText, _ := os.ReadFile(s.configPath)
		subText, _ := os.ReadFile(cfg.Subscription.File)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"config":        string(cfgText),
			"subscriptions": string(subText),
		})
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		newConfig := r.FormValue("config_text")
		if strings.TrimSpace(newConfig) == "" {
			http.Error(w, "config_text is required", http.StatusBadRequest)
			return
		}
		originalConfig, _ := os.ReadFile(s.configPath)
		if err := os.WriteFile(s.configPath, []byte(newConfig), 0644); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		loaded, err := config.Load(s.configPath)
		if err != nil {
			_ = os.WriteFile(s.configPath, originalConfig, 0644)
			http.Error(w, fmt.Sprintf("config validation failed: %v", err), http.StatusBadRequest)
			return
		}
		subText := r.FormValue("subscriptions_text")
		if subText != "" || loaded.Subscription.File != "" {
			originalSubs, _ := os.ReadFile(loaded.Subscription.File)
			if err := os.WriteFile(loaded.Subscription.File, []byte(subText), 0644); err != nil {
				_ = os.WriteFile(s.configPath, originalConfig, 0644)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if _, err := config.Load(s.configPath); err != nil {
				_ = os.WriteFile(s.configPath, originalConfig, 0644)
				_ = os.WriteFile(loaded.Subscription.File, originalSubs, 0644)
				http.Error(w, fmt.Sprintf("reload validation failed: %v", err), http.StatusBadRequest)
				return
			}
		}
		_, _ = w.Write([]byte("saved"))
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, err := config.Load(s.configPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if token, ok := s.authenticated(r, cfg.Web.AuthToken); !ok || token == "" {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func (s *Server) authenticated(r *http.Request, expected string) (string, bool) {
	cookie, err := r.Cookie("nodes_check_token")
	if err == nil && cookie.Value == expected {
		return cookie.Value, true
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(auth, "Bearer ") {
		token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		if token == expected {
			return token, true
		}
	}
	return "", false
}

const loginHTML = `<!doctype html>
<html lang="zh-CN">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>nodes-check login</title>
<style>body{font-family:Segoe UI,system-ui,sans-serif;background:#0b1220;color:#e5eef7;display:grid;place-items:center;min-height:100vh;margin:0}.card{background:#131c2f;padding:24px;border-radius:16px;box-shadow:0 20px 50px rgba(0,0,0,.35);width:min(420px,90vw)}input,button{width:100%;padding:12px 14px;border-radius:10px;border:1px solid #334155;margin-top:12px}input{background:#0f172a;color:#fff}button{background:#2563eb;color:#fff;border:none;cursor:pointer}</style></head>
<body><form class="card" method="post" action="/login"><h2>nodes-check 登录</h2><p>请输入访问 token。</p><input type="password" name="token" placeholder="Auth token" autofocus><button type="submit">登录</button></form></body></html>`

const indexHTML = `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>nodes-check</title>
<style>
:root{--bg:#07111f;--panel:#0f1b2d;--soft:#16253d;--text:#e8eef7;--muted:#96a8c2;--line:#28405e;--accent:#38bdf8;--good:#22c55e;--warn:#f59e0b;--bad:#ef4444}
*{box-sizing:border-box}body{margin:0;font-family:Segoe UI,system-ui,sans-serif;background:radial-gradient(circle at top,#10213f,#07111f 55%);color:var(--text)}
header{padding:24px 28px;display:flex;justify-content:space-between;align-items:center}main{padding:0 28px 28px;display:grid;grid-template-columns:1.1fr .9fr;gap:20px}section{background:rgba(15,27,45,.9);border:1px solid var(--line);border-radius:18px;padding:18px}h1,h2,h3{margin:0 0 12px}pre{white-space:pre-wrap;word-break:break-word;background:#08111f;padding:14px;border-radius:12px;border:1px solid #20324d;max-height:320px;overflow:auto}textarea{width:100%;min-height:260px;background:#08111f;color:#fff;border:1px solid #28405e;border-radius:12px;padding:12px;font:13px Consolas,monospace}button{background:var(--accent);color:#062033;border:none;border-radius:10px;padding:10px 14px;font-weight:700;cursor:pointer}button.secondary{background:#1e293b;color:#e5eef7}.badge{display:inline-block;padding:4px 10px;border-radius:999px;background:#12314a;color:#9fe7ff;font-size:12px;margin-right:8px}.grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}.metric{padding:14px;background:#0a1527;border:1px solid #223450;border-radius:14px}.muted{color:var(--muted)}.row{display:flex;gap:10px;align-items:center;flex-wrap:wrap}.cols{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:20px}.small{font-size:12px}.top-actions form{display:inline-block;margin-right:8px}
</style>
<script>
async function refreshState(){const res=await fetch('/api/state'); if(!res.ok)return; const s=await res.json(); document.getElementById('stage').textContent=s.stage||'idle'; document.getElementById('message').textContent=s.message||''; document.getElementById('running').textContent=s.running?'运行中':'空闲'; document.getElementById('error').textContent=s.last_error||'无'; document.getElementById('duration').textContent=s.last_duration||'-'; document.getElementById('output').textContent=s.last_output_path||'-'; document.getElementById('history').textContent=s.last_history_path||'-'; document.getElementById('counters').textContent=JSON.stringify(s.counters||{},null,2); document.getElementById('categories').textContent=JSON.stringify(s.category_counts||{},null,2); document.getElementById('logs').textContent=(s.recent_logs||[]).join('\n'); }
async function runNow(){const res=await fetch('/api/run',{method:'POST'}); if(!res.ok){alert(await res.text()); return;} refreshState();}
async function saveConfig(ev){ev.preventDefault(); const form=new FormData(ev.target); const res=await fetch('/api/config',{method:'POST',body:new URLSearchParams(form)}); const text=await res.text(); if(!res.ok){alert(text); return;} alert(text);}
setInterval(refreshState,4000); window.addEventListener('load',refreshState);
</script>
</head>
<body>
<header><div><h1>nodes-check</h1><div class="muted">配置编辑、任务状态、手动执行</div></div><div class="top-actions"><button onclick="runNow()">立即运行</button><form method="post" action="/logout"><button type="submit" class="secondary">退出</button></form></div></header>
<main>
<section>
<h2>任务状态</h2>
<div class="grid">
<div class="metric"><div class="muted small">状态</div><div id="running">{{if .State.Running}}运行中{{else}}空闲{{end}}</div></div>
<div class="metric"><div class="muted small">阶段</div><div id="stage">{{.State.Stage}}</div></div>
<div class="metric"><div class="muted small">当前消息</div><div id="message">{{.State.Message}}</div></div>
<div class="metric"><div class="muted small">最近耗时</div><div id="duration">{{.State.LastDuration}}</div></div>
<div class="metric"><div class="muted small">输出文件</div><div id="output">{{.State.LastOutputPath}}</div></div>
<div class="metric"><div class="muted small">历史池</div><div id="history">{{.State.LastHistoryPath}}</div></div>
</div>
<div class="cols" style="margin-top:16px">
<div><h3>计数</h3><pre id="counters">{{printf "%#v" .State.Counters}}</pre></div>
<div><h3>分类</h3><pre id="categories">{{printf "%#v" .State.CategoryCounts}}</pre></div>
</div>
<div style="margin-top:16px"><h3>最近日志</h3><pre id="logs">{{range .State.RecentLogs}}{{.}}
{{end}}</pre></div>
<div style="margin-top:16px"><h3>最近错误</h3><pre id="error">{{if .State.LastError}}{{.State.LastError}}{{else}}无{{end}}</pre></div>
</section>
<section>
<h2>配置编辑</h2>
<form onsubmit="saveConfig(event)">
<div class="row"><span class="badge">config</span><span class="muted small">{{.ConfigPath}}</span></div>
<textarea name="config_text">{{.ConfigText}}</textarea>
<div class="row" style="margin-top:14px"><span class="badge">subscriptions</span></div>
<textarea name="subscriptions_text">{{.Subscriptions}}</textarea>
<div style="margin-top:14px"><button type="submit">保存配置</button></div>
</form>
</section>
</main>
</body></html>`
