import { useEffect, useMemo, useState } from 'react'
import {
  CheckCircle2, Check, Copy, FilePlus, Plug, Plus, Save, Star, StarOff,
  Trash2, Wand2, Wrench,
} from 'lucide-react'
import { AI, MCP, mcpEndpoint, type AISettings, type MCPToken, type PromptTemplate } from '@/lib/api'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter,
} from '@/components/ui/dialog'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { Button } from '@/components/ui/button'
import { Input, Textarea } from '@/components/ui/input'
import { cn, copyToClipboard } from '@/lib/utils'

const PRESETS = [
  { name: 'OpenAI', baseUrl: 'https://api.openai.com/v1', model: 'gpt-4o-mini' },
  { name: 'DeepSeek', baseUrl: 'https://api.deepseek.com/v1', model: 'deepseek-chat' },
  { name: 'Moonshot Kimi', baseUrl: 'https://api.moonshot.cn/v1', model: 'moonshot-v1-32k' },
  { name: '智谱 GLM', baseUrl: 'https://open.bigmodel.cn/api/paas/v4', model: 'glm-4-flash' },
  { name: '阿里 通义', baseUrl: 'https://dashscope.aliyuncs.com/compatible-mode/v1', model: 'qwen-plus' },
  { name: 'OpenRouter', baseUrl: 'https://openrouter.ai/api/v1', model: 'anthropic/claude-3.5-sonnet' },
  { name: '自定义', baseUrl: '', model: '' },
]

export function AISettingsDialog({
  open, onOpenChange,
}: { open: boolean; onOpenChange: (v: boolean) => void }) {
  const [s, setS] = useState<AISettings | null>(null)
  const [busy, setBusy] = useState(false)
  const [tab, setTab] = useState<'connection' | 'skills' | 'mcp'>('connection')

  useEffect(() => {
    if (open) AI.getSettings().then((r) => setS(r.settings))
  }, [open])

  if (!s) return null

  const set = <K extends keyof AISettings>(k: K, v: AISettings[K]) => setS({ ...s, [k]: v })
  const applyPreset = (p: typeof PRESETS[number]) => setS({ ...s, baseUrl: p.baseUrl, model: p.model })

  const save = async () => {
    setBusy(true)
    try {
      const apiKey = s.apiKey?.includes('•') ? '' : s.apiKey
      await AI.updateSettings({ ...s, apiKey })
      onOpenChange(false)
    } catch (e: any) {
      alert(e?.response?.data?.error ?? e?.message ?? '保存失败')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-3xl max-h-[85vh] grid-rows-[auto_minmax(0,1fr)_auto]">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Wand2 className="h-5 w-5 text-violet-400" /> AI 设置
          </DialogTitle>
          <DialogDescription>
            兼容 OpenAI Chat Completions 协议。
            <b className="text-foreground">Skill</b> 是预制提示词（场景模板），
            <b className="text-foreground">Tool</b> 是 AI 真正调用的文件读写能力（list_files / read_file / write_file / replace_in_file）。
          </DialogDescription>
        </DialogHeader>

        <Tabs value={tab} onValueChange={(v) => setTab(v as any)} className="flex flex-col min-h-0">
          <TabsList className="w-full justify-start shrink-0">
            <TabsTrigger value="connection">连接 / 模型</TabsTrigger>
            <TabsTrigger value="skills">Skill 管理</TabsTrigger>
            <TabsTrigger value="mcp">MCP 接入</TabsTrigger>
          </TabsList>

          {/* ---------------- 连接 ---------------- */}
          <TabsContent value="connection" className="space-y-4 py-3 flex-1 min-h-0 overflow-y-auto">
            <div>
              <label className="text-xs text-muted-foreground mb-1.5 block">服务商预设</label>
              <div className="flex flex-wrap gap-1.5">
                {PRESETS.map((p) => (
                  <button
                    key={p.name}
                    onClick={() => applyPreset(p)}
                    className="text-xs rounded border border-border/60 px-2 py-1 hover:border-primary/60 hover:bg-accent/50 transition-colors"
                  >
                    {p.name}
                  </button>
                ))}
              </div>
            </div>

            <Field label="Base URL" hint="OpenAI 兼容 API 的 base url">
              <Input value={s.baseUrl} onChange={(e) => set('baseUrl', e.target.value)} placeholder="https://api.openai.com/v1" />
            </Field>

            <Field label="API Key" hint="存储在本地服务，不会上传到第三方">
              <Input type="password" value={s.apiKey} onChange={(e) => set('apiKey', e.target.value)} placeholder="sk-..." />
            </Field>

            <Field label="Model">
              <Input value={s.model} onChange={(e) => set('model', e.target.value)} placeholder="gpt-4o-mini" />
            </Field>

            <div className="grid grid-cols-3 gap-3">
              <Field label="Temperature">
                <Input
                  type="number" step="0.1" min="0" max="2"
                  value={s.temperature}
                  onChange={(e) => set('temperature', Number(e.target.value))}
                />
              </Field>
              <Field label="Max Tokens">
                <Input
                  type="number" min="256"
                  value={s.maxTokens}
                  onChange={(e) => set('maxTokens', Number(e.target.value))}
                />
              </Field>
              <Field label="Tool Rounds">
                <Input
                  type="number" min="1" max="20"
                  value={s.maxToolRounds}
                  onChange={(e) => set('maxToolRounds', Number(e.target.value))}
                />
              </Field>
            </div>

            <div className="rounded-md border border-border/60 p-3 flex items-start gap-3">
              <Wrench className="h-4 w-4 text-violet-400 mt-0.5" />
              <div className="flex-1">
                <div className="text-sm font-medium">启用 Tool Calling（修改场景）</div>
                <div className="text-[11px] text-muted-foreground mt-0.5">
                  开启后，修改文档时 AI 会通过 list_files / read_file / write_file / replace_in_file 工具按需读取和编辑文件，
                  避免把整个文档塞给模型。仅 OpenAI 兼容、且模型支持 function calling 才能生效。
                </div>
              </div>
              <input
                type="checkbox"
                checked={s.enableTools !== false}
                onChange={(e) => set('enableTools', e.target.checked)}
                className="mt-1 h-4 w-4 accent-violet-500"
              />
            </div>
          </TabsContent>

          {/* ---------------- Skill 管理（即 Prompt 模板） ---------------- */}
          <TabsContent value="skills" className="py-3 flex-1 min-h-0 overflow-y-auto">
            <PromptManager />
          </TabsContent>

          {/* ---------------- MCP 接入 ---------------- */}
          <TabsContent value="mcp" className="py-3 flex-1 min-h-0 overflow-y-auto">
            <MCPPanel />
          </TabsContent>
        </Tabs>

        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>关闭</Button>
          {tab === 'connection' && (
            <Button variant="gradient" onClick={save} disabled={busy}>
              <Save /> {busy ? '保存中…' : '保存'}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function Field({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <div>
      <label className="text-xs text-muted-foreground mb-1.5 block">{label}</label>
      {children}
      {hint && <div className="text-[10px] text-muted-foreground/70 mt-1">{hint}</div>}
    </div>
  )
}

// ===================== Skill 管理（基于 Prompt 模板） =====================
// Skill = 一组预制的提示词模板（按场景分：创建 / 修改）。
// 它和 Tool 是两个独立概念：Tool 是 AI 调用的实际写文件能力。

function PromptManager() {
  const [items, setItems] = useState<PromptTemplate[]>([])
  const [activeId, setActiveId] = useState<string | null>(null)
  const [editing, setEditing] = useState<PromptTemplate | null>(null)
  const [busy, setBusy] = useState(false)

  const reload = async () => {
    const list = await AI.listPrompts()
    setItems(list)
    if (!activeId && list.length) setActiveId(list[0].id)
  }
  useEffect(() => { reload() /* eslint-disable-next-line */ }, [])

  useEffect(() => {
    if (!activeId) { setEditing(null); return }
    const found = items.find((i) => i.id === activeId)
    if (found) setEditing({ ...found })
  }, [activeId, items])

  const grouped = useMemo(() => ({
    create: items.filter((i) => i.scene === 'create'),
    edit: items.filter((i) => i.scene === 'edit'),
  }), [items])

  const newPrompt = (scene: 'create' | 'edit') => {
    setEditing({
      id: '__new__',
      name: scene === 'create' ? '新创建模板' : '新修改模板',
      scene,
      content: '',
      builtin: false,
      isDefault: false,
      createdAt: '',
      updatedAt: '',
    })
    setActiveId(null)
  }

  const saveCurrent = async () => {
    if (!editing) return
    setBusy(true)
    try {
      let saved: PromptTemplate
      if (editing.id === '__new__') {
        saved = await AI.createPrompt({
          name: editing.name, scene: editing.scene,
          content: editing.content, isDefault: editing.isDefault,
        })
      } else {
        saved = await AI.updatePrompt(editing.id, {
          name: editing.name, scene: editing.scene,
          content: editing.content, isDefault: editing.isDefault,
        })
      }
      await reload()
      setActiveId(saved.id)
    } catch (e: any) {
      alert(e?.response?.data?.error ?? e?.message ?? '保存失败')
    } finally {
      setBusy(false)
    }
  }

  const removeCurrent = async () => {
    if (!editing || editing.builtin || editing.id === '__new__') return
    if (!confirm(`删除 Skill「${editing.name}」？`)) return
    setBusy(true)
    try {
      await AI.deletePrompt(editing.id)
      await reload()
      setActiveId(items[0]?.id ?? null)
    } catch (e: any) {
      alert(e?.response?.data?.error ?? e?.message ?? '删除失败')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="grid grid-cols-[220px_1fr] gap-3 h-[480px] min-h-0">
      {/* 列表 */}
      <div className="border border-border/60 rounded-md overflow-hidden flex flex-col min-h-0">
        <div className="p-2 border-b border-border/60 flex items-center gap-1">
          <Button variant="ghost" size="sm" className="flex-1 h-7 text-xs" onClick={() => newPrompt('create')}>
            <FilePlus /> 新建创建 Skill
          </Button>
          <Button variant="ghost" size="sm" className="flex-1 h-7 text-xs" onClick={() => newPrompt('edit')}>
            <FilePlus /> 新建修改 Skill
          </Button>
        </div>
        <div className="flex-1 overflow-y-auto py-1">
          <Group label="创建场景 Skill" list={grouped.create} activeId={activeId} onSelect={setActiveId} />
          <Group label="修改场景 Skill" list={grouped.edit} activeId={activeId} onSelect={setActiveId} />
        </div>
      </div>

      {/* 编辑 */}
      <div className="flex flex-col min-h-0">
        {editing ? (
          <div className="flex-1 flex flex-col gap-2 min-h-0">
            <div className="flex items-center gap-2">
              <Input
                value={editing.name}
                onChange={(e) => setEditing({ ...editing, name: e.target.value })}
                disabled={editing.builtin}
                className="text-sm"
              />
              <select
                value={editing.scene}
                onChange={(e) => setEditing({ ...editing, scene: e.target.value as any })}
                disabled={editing.builtin}
                className="bg-background border border-border/60 rounded h-9 px-2 text-xs"
              >
                <option value="create">创建</option>
                <option value="edit">修改</option>
              </select>
              <Button
                variant={editing.isDefault ? 'gradient' : 'ghost'}
                size="sm" className="h-9 text-xs"
                onClick={() => setEditing({ ...editing, isDefault: !editing.isDefault })}
                title={editing.isDefault ? '当前为默认' : '设为默认'}
              >
                {editing.isDefault ? <Star /> : <StarOff />}
                {editing.isDefault ? '默认' : '设默认'}
              </Button>
            </div>
            <Textarea
              rows={18}
              value={editing.content}
              onChange={(e) => setEditing({ ...editing, content: e.target.value })}
              className="font-mono text-xs flex-1 resize-none"
              placeholder="输入 Prompt 内容…"
            />
            <div className="flex items-center gap-2">
              <span className="text-[11px] text-muted-foreground flex-1">
                {editing.builtin
                  ? '内置模板：可修改名称/内容；不能删除。'
                  : '自定义模板'}
              </span>
              {!editing.builtin && editing.id !== '__new__' && (
                <Button variant="ghost" size="sm" onClick={removeCurrent} disabled={busy}>
                  <Trash2 /> 删除
                </Button>
              )}
              <Button variant="gradient" size="sm" onClick={saveCurrent} disabled={busy}>
                <Save /> {busy ? '保存中…' : '保存'}
              </Button>
            </div>
          </div>
        ) : (
          <div className="flex-1 flex items-center justify-center text-xs text-muted-foreground">
            选择左侧模板进行编辑，或点击「+ 创建型 / 修改型」新建。
          </div>
        )}
      </div>
    </div>
  )
}

function Group({
  label, list, activeId, onSelect,
}: {
  label: string
  list: PromptTemplate[]
  activeId: string | null
  onSelect: (id: string) => void
}) {
  if (list.length === 0) return null
  return (
    <div className="px-1.5 py-1">
      <div className="text-[10px] uppercase tracking-wider text-muted-foreground px-1.5 py-1">
        {label}
      </div>
      <div className="space-y-0.5">
        {list.map((p) => (
          <button
            key={p.id}
            onClick={() => onSelect(p.id)}
            className={cn(
              'w-full text-left rounded px-2 py-1.5 transition-colors flex items-center gap-1.5',
              activeId === p.id ? 'bg-accent text-foreground' : 'hover:bg-muted/50',
            )}
          >
            <span className="text-xs truncate flex-1">{p.name}</span>
            {p.isDefault && <CheckCircle2 className="h-3 w-3 text-emerald-400 shrink-0" />}
            {p.builtin && <span className="text-[10px] text-violet-300 shrink-0">内置</span>}
          </button>
        ))}
      </div>
    </div>
  )
}

// ===================== MCP 接入面板 =====================

function MCPPanel() {
  const endpoint = mcpEndpoint()
  const [tokens, setTokens] = useState<MCPToken[]>([])
  const [busy, setBusy] = useState(false)
  const [newName, setNewName] = useState('default')
  // 新生成的明文 token 仅展示一次
  const [revealed, setRevealed] = useState<MCPToken | null>(null)
  const [copied, setCopied] = useState<string>('')

  const reload = async () => {
    try {
      const items = await MCP.listTokens()
      setTokens(items)
    } catch (e: any) {
      // 忽略
    }
  }
  useEffect(() => { reload() }, [])

  const create = async () => {
    setBusy(true)
    try {
      const t = await MCP.createToken(newName.trim() || 'default')
      setRevealed(t)
      await reload()
    } catch (e: any) {
      alert(e?.response?.data?.error ?? e?.message ?? '创建失败')
    } finally {
      setBusy(false)
    }
  }
  const remove = async (id: string) => {
    if (!confirm('删除该 Token？删除后所有使用此 Token 的客户端将失效。')) return
    setBusy(true)
    try {
      await MCP.deleteToken(id)
      await reload()
    } catch (e: any) {
      alert(e?.response?.data?.error ?? e?.message ?? '删除失败')
    } finally {
      setBusy(false)
    }
  }

  const copy = async (key: string, text: string) => {
    const ok = await copyToClipboard(text)
    if (ok) {
      setCopied(key)
      setTimeout(() => setCopied(''), 1500)
    } else {
      // 兜底：在非 HTTPS 等无法访问 Clipboard API 的环境下，弹出 prompt 让用户手动复制
      window.prompt('复制失败，请手动按 Ctrl/Cmd+C 复制：', text)
    }
  }

  // 用于配置示例的 token：优先用刚生成的明文；否则提示用户先创建
  const tokenForExample = revealed?.token ?? '<YOUR_TOKEN>'

  // Cursor / Claude Desktop / Cline 配置示例：使用 mcp-remote 桥接
  const remoteJSON = JSON.stringify({
    mcpServers: {
      'web-doc': {
        command: 'npx',
        args: ['-y', 'mcp-remote', endpoint, '--header', `Authorization: Bearer ${tokenForExample}`],
      },
    },
  }, null, 2)

  // 直连（部分客户端原生支持 Streamable HTTP）
  const directJSON = JSON.stringify({
    mcpServers: {
      'web-doc': {
        url: endpoint,
        headers: { Authorization: `Bearer ${tokenForExample}` },
      },
    },
  }, null, 2)

  return (
    <div className="space-y-4">
      {/* 端点信息 */}
      <div className="rounded-md border border-border/60 p-3">
        <div className="flex items-center gap-2 mb-2">
          <Plug className="h-4 w-4 text-violet-400" />
          <div className="text-sm font-medium">MCP 服务端点</div>
        </div>
        <div className="flex items-center gap-2">
          <code className="flex-1 bg-muted/40 rounded px-2 py-1.5 text-xs font-mono break-all">
            {endpoint}
          </code>
          <Button variant="ghost" size="sm" onClick={() => copy('ep', endpoint)}>
            {copied === 'ep' ? <Check /> : <Copy />}
          </Button>
        </div>
        <div className="text-[11px] text-muted-foreground mt-2 leading-relaxed">
          基于 <a className="underline hover:text-foreground" href="https://modelcontextprotocol.io" target="_blank" rel="noreferrer">Model Context Protocol</a>{' '}
          Streamable HTTP（JSON-RPC 2.0），用于 AI Agent 直接读取/创建/更新文档。
          <br />鉴权方式：HTTP Header <code className="font-mono">Authorization: Bearer &lt;token&gt;</code>
        </div>
      </div>

      {/* Token 管理 */}
      <div className="rounded-md border border-border/60 p-3">
        <div className="text-sm font-medium mb-2">访问 Token</div>

        <div className="flex items-center gap-2 mb-3">
          <Input
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            placeholder="给 Token 起个名字（如 cursor-mac）"
            className="flex-1 h-9 text-xs"
          />
          <Button variant="gradient" size="sm" onClick={create} disabled={busy}>
            <Plus /> 生成 Token
          </Button>
        </div>

        {/* 明文显示（仅刚生成时） */}
        {revealed && (
          <div className="rounded border border-emerald-500/40 bg-emerald-500/5 p-2.5 mb-3">
            <div className="text-[11px] text-emerald-300 mb-1">
              ⚠️ 这是 Token 明文，<b>仅本次显示</b>，请立即复制保存：
            </div>
            <div className="flex items-center gap-2">
              <code className="flex-1 bg-background/40 rounded px-2 py-1 text-xs font-mono break-all">
                {revealed.token}
              </code>
              <Button variant="ghost" size="sm" onClick={() => copy('plain', revealed.token)}>
                {copied === 'plain' ? <Check /> : <Copy />}
              </Button>
              <Button variant="ghost" size="sm" onClick={() => setRevealed(null)}>关闭</Button>
            </div>
          </div>
        )}

        {/* 列表 */}
        <div className="space-y-1">
          {tokens.length === 0 && (
            <div className="text-xs text-muted-foreground py-2">尚未创建任何 Token。</div>
          )}
          {tokens.map((t) => (
            <div key={t.id} className="flex items-center gap-2 px-2 py-1.5 rounded bg-muted/30">
              <span className="text-xs flex-1 truncate">{t.name}</span>
              <code className="text-[11px] font-mono text-muted-foreground">{t.token}</code>
              <span className="text-[10px] text-muted-foreground">
                {t.lastUsedAt
                  ? `用过 · ${new Date(t.lastUsedAt).toLocaleDateString()}`
                  : '未使用'}
              </span>
              <Button variant="ghost" size="sm" onClick={() => remove(t.id)} disabled={busy}>
                <Trash2 />
              </Button>
            </div>
          ))}
        </div>
      </div>

      {/* 客户端配置示例 */}
      <div className="rounded-md border border-border/60 p-3 space-y-3">
        <div className="text-sm font-medium">客户端配置</div>

        <div>
          <div className="flex items-center justify-between mb-1.5">
            <div className="text-xs">
              方式 A：<b>mcp-remote 桥接</b>（推荐 / Claude Desktop · Cursor · Cline 通用）
            </div>
            <Button variant="ghost" size="sm" onClick={() => copy('remote', remoteJSON)}>
              {copied === 'remote' ? <Check /> : <Copy />} 复制
            </Button>
          </div>
          <pre className="bg-muted/40 rounded p-2 text-[11px] font-mono overflow-auto max-h-48 whitespace-pre">
{remoteJSON}
          </pre>
          <div className="text-[10px] text-muted-foreground mt-1">
            写入 Claude Desktop 的 <code>claude_desktop_config.json</code> 或 Cursor 的 <code>~/.cursor/mcp.json</code>。需先安装 Node.js。
          </div>
        </div>

        <div>
          <div className="flex items-center justify-between mb-1.5">
            <div className="text-xs">
              方式 B：<b>直连</b>（支持 Streamable HTTP 的客户端）
            </div>
            <Button variant="ghost" size="sm" onClick={() => copy('direct', directJSON)}>
              {copied === 'direct' ? <Check /> : <Copy />} 复制
            </Button>
          </div>
          <pre className="bg-muted/40 rounded p-2 text-[11px] font-mono overflow-auto max-h-40 whitespace-pre">
{directJSON}
          </pre>
        </div>

        {!revealed && (
          <div className="text-[11px] text-amber-400/90">
            提示：上面示例中的 <code>&lt;YOUR_TOKEN&gt;</code> 需要替换成你创建 Token 时获得的明文。
          </div>
        )}
      </div>

      {/* 工具清单 */}
      <div className="rounded-md border border-border/60 p-3">
        <div className="text-sm font-medium mb-2">可用工具</div>
        <ul className="text-[11px] text-muted-foreground space-y-1 leading-relaxed">
          <li><code className="font-mono text-foreground">list_documents</code> — 列出全部文档/文件夹</li>
          <li><code className="font-mono text-foreground">get_document</code> — 获取文档元信息和文件清单</li>
          <li><code className="font-mono text-foreground">read_document_file</code> — 读取文档内某个文件文本</li>
          <li><code className="font-mono text-foreground">create_document</code> — 创建新文档/文件夹</li>
          <li><code className="font-mono text-foreground">upload_html</code> — 写入/覆盖单个 HTML 文件</li>
          <li><code className="font-mono text-foreground">upload_zip_base64</code> — 上传 base64 编码的 zip 整站</li>
          <li><code className="font-mono text-foreground">delete_document</code> — 删除文档/文件夹</li>
        </ul>
      </div>
    </div>
  )
}
