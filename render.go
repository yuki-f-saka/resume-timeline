package main

import (
	"fmt"
	"html/template"
	"os"
)

type ColumnView struct {
	Meta     string
	Subject  string
	PNGB64   string
	Overlays []Overlay
}

type PageData struct {
	File string
	Cols []ColumnView
}

var funcs = template.FuncMap{
	"pct": func(f float64) template.CSS {
		return template.CSS(fmt.Sprintf("%.3f%%", f*100))
	},
}

var tpl = template.Must(template.New("page").Funcs(funcs).Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Resume Timeline — {{.File}}</title>
<style>
:root { --colw: clamp(240px, 20vw, 480px); }
* { box-sizing: border-box; }
html, body { height: 100%; }
body { margin: 0; display: flex; flex-direction: column;
  font: 13px/1.5 -apple-system, "Segoe UI", sans-serif;
  background: #f6f8fa; color: #1f2328; }
.toolbar { display: flex; gap: 18px; align-items: center; flex-wrap: wrap;
  padding: 8px 14px; background: #24292f; color: #fff; }
.toolbar .file { color: #8b949e; }
.toolbar label { display: flex; align-items: center; gap: 6px; }
.legend { display: flex; align-items: center; gap: 12px; color: #d0d7de; }
.legend i { display: inline-block; width: 14px; height: 14px; border-radius: 3px; margin-right: 4px; vertical-align: -2px; }
.l-add { background: #aceebb; }
.l-chg { background: #6fdd8b; }
.l-del { background: #cf222e; height: 4px !important; }
.cols { flex: 1; display: flex; overflow-x: auto; align-items: flex-start; }
.col { flex: 0 0 var(--colw); width: var(--colw); height: 100%;
  border-right: 1px solid #d0d7de; background: #fff; overflow-y: auto; }
.colhead { position: sticky; top: 0; background: #fff;
  border-bottom: 2px solid #d0d7de; padding: 6px 10px; z-index: 4; }
.meta { font-weight: 600; }
.subj { color: #57606a; font-size: 12px; word-break: break-word; }
.page { position: relative; }
.pageimg { display: block; width: 100%; }
.ov { position: absolute; left: 0; width: 100%; pointer-events: none; }
/* GitHub-style diff colors: added line = light green, changed regions = darker
   green, deletion = red marker */
.ov.add { background: rgba(46,160,67,.16); border-left: 3px solid #1f883d; }
.ov.chg { background: rgba(46,160,67,.08); border-left: 3px dotted #1f883d; }
.ov.chg .box { position: absolute; top: 0; height: 100%;
  background: rgba(74,194,107,.40); outline: 1px solid rgba(31,136,61,.45); }
.ov.del { background: #cf222e; pointer-events: auto; cursor: help; z-index: 3; }
.ov.del .ghost { display: none; position: absolute; top: 5px; left: 0; width: 100%;
  border: 2px solid #cf222e; background: #ffebe9; padding: 2px 0; z-index: 5; }
.ov.del:hover .ghost { display: block; }
</style>
</head>
<body>
<div class="toolbar">
  <strong>Resume Timeline</strong>
  <span class="file">{{.File}} · {{len .Cols}} versions</span>
  <label>Column width <input type="range" id="colw" min="200" max="720" step="10"></label>
  <label class="sync"><input type="checkbox" id="syncscroll" checked> Sync scroll</label>
  <span class="legend">
    <span><i class="l-add"></i>Added</span>
    <span><i class="l-chg"></i>Changed region</span>
    <span><i class="l-del"></i>Deleted (hover to peek)</span>
  </span>
</div>
<div class="cols">
{{range .Cols}}  <div class="col">
    <div class="colhead">
      <div class="meta">{{.Meta}}</div>
      {{if .Subject}}<div class="subj">{{.Subject}}</div>{{end}}
    </div>
    <div class="page">
      <img class="pageimg" src="data:image/png;base64,{{.PNGB64}}" loading="lazy">
{{range .Overlays}}{{if eq .Kind "del"}}      <div class="ov del" style="top:{{pct .Top}};height:{{pct .Height}}"><img class="ghost" src="data:image/png;base64,{{.GhostB64}}"></div>
{{else}}      <div class="ov {{.Kind}}" style="top:{{pct .Top}};height:{{pct .Height}}">{{range .Boxes}}<i class="box" style="left:{{pct .X0}};width:{{pct .W}}"></i>{{end}}</div>
{{end}}{{end}}    </div>
  </div>
{{end}}</div>
<script>
const slider = document.getElementById('colw');
const cols = document.querySelector('.cols');
// default: the two most recent versions side by side, scrolled to the newest
const half = Math.floor(cols.clientWidth / 2) - 1;
slider.max = Math.max(Number(slider.max), half);
slider.value = half;
document.documentElement.style.setProperty('--colw', half + 'px');
cols.scrollLeft = cols.scrollWidth;

// headers grow with long commit messages; equalize their heights so page
// images stay vertically aligned across columns
const heads = document.querySelectorAll('.colhead');
function alignHeads() {
  let maxH = 0;
  heads.forEach(h => { h.style.minHeight = ''; });
  heads.forEach(h => { maxH = Math.max(maxH, h.offsetHeight); });
  heads.forEach(h => { h.style.minHeight = maxH + 'px'; });
}
alignHeads();

slider.addEventListener('input', e => {
  document.documentElement.style.setProperty('--colw', e.target.value + 'px');
  alignHeads();
});

// while enabled, scrolling one column scrolls every column
const syncBox = document.getElementById('syncscroll');
let syncing = false;
document.querySelectorAll('.col').forEach(col => {
  col.addEventListener('scroll', () => {
    if (!syncBox.checked || syncing) return;
    syncing = true;
    document.querySelectorAll('.col').forEach(other => {
      if (other !== col) other.scrollTop = col.scrollTop;
    });
    requestAnimationFrame(() => { syncing = false; });
  });
});
</script>
</body>
</html>
`))

func renderHTML(path string, data PageData) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return tpl.Execute(f, data)
}
