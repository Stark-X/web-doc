import { useEffect, useState } from 'react'
import { Check, Copy, Link } from 'lucide-react'
import { Shares, type DocNode, type ShareInfo } from '@/lib/api'
import { copyToClipboard } from '@/lib/utils'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

export function ShareDialog({
  doc, open, onOpenChange,
}: {
  doc: DocNode | null
  open: boolean
  onOpenChange: (v: boolean) => void
}) {
  const [share, setShare] = useState<ShareInfo | null>(null)
  const [copiedKey, setCopiedKey] = useState<string | null>(null)

  useEffect(() => {
    if (open && doc) {
      Shares.create(doc.id).then((s) => setShare(s))
    } else {
      setShare(null); setCopiedKey(null)
    }
  }, [open, doc])

  // 默认分享链接：/s/:token 打开后会自动跳转到 /v/:docId（带完整主站外壳）。
  // 访问者如需隐藏主站顶部和左侧菜单，可使用 ?fullscreen 链接（仍然是 React 外壳 + iframe，仅视觉隐藏菜单）。
  // 静态页面链接：后端 /p/:token 直接输出文档文件（无外壳、无 iframe），响应带
  // CSP sandbox，文档脚本运行在 opaque origin，读不到访问者在主站的登录态。
  // 未配置分享专用域名时，必须拼上 Vite 构建时的 BASE_URL（如 /doc/），否则反向代理子路径部署会 404。
  const baseUrl = (import.meta.env.BASE_URL || '/').replace(/\/+$/, '')
  const entryPath = (doc?.entryFile || 'index.html').split('/').map(encodeURIComponent).join('/')
  const token = share?.token
  const staticShareBase = share?.staticShareBaseUrl?.replace(/\/+$/, '') || `${location.origin}${baseUrl}/p`
  const links = token ? [
    {
      key: 'default',
      url: `${location.origin}${baseUrl}/s/${token}`,
      hint: '默认链接：访问者会看到完整的文档站（顶部＋左侧菜单）。',
    },
    {
      key: 'fullscreen',
      url: `${location.origin}${baseUrl}/s/${token}?fullscreen=1`,
      hint: '全屏链接（?fullscreen）：隐藏主站顶部和左侧菜单，只展示文档内容（仍保留 iframe 隔离）。',
    },
    {
      key: 'static',
      url: `${staticShareBase}/${token}/${entryPath}`,
      hint: '静态页面链接：直接打开文档本身，无主站外壳；浏览器沙箱隔离，文档脚本无法读取访问者的登录状态。',
    },
  ] : []

  const copy = async (key: string, target: string) => {
    if (!target) return
    const ok = await copyToClipboard(target)
    if (ok) {
      setCopiedKey(key)
      setTimeout(() => setCopiedKey((k) => (k === key ? null : k)), 1600)
    } else {
      // 兜底：复制失败时提示用户手动选中复制（多见于非 HTTPS 站点 + 浏览器禁用了 execCommand）
      window.prompt('复制失败，请手动按 Ctrl/Cmd+C 复制：', target)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Link className="h-5 w-5 text-primary" />
            分享 "{doc?.title}"
          </DialogTitle>
          <DialogDescription>
            通过链接分享这个文档，访问者无需登录即可查看。
          </DialogDescription>
        </DialogHeader>
        {links.map((l, i) => (
          <div key={l.key}>
            <div className="flex items-center gap-2 pt-2">
              <Input value={l.url} readOnly className="font-mono text-xs" />
              <Button
                onClick={() => copy(l.key, l.url)}
                variant={copiedKey === l.key ? 'default' : i === 0 ? 'gradient' : 'outline'}
              >
                {copiedKey === l.key ? <><Check /> 已复制</> : <><Copy /> 复制</>}
              </Button>
            </div>
            <p className="text-xs text-muted-foreground pt-1">{l.hint}</p>
          </div>
        ))}
        {!token && (
          <div className="flex items-center gap-2 pt-2">
            <Input value="" readOnly placeholder="生成中…" className="font-mono text-xs" />
            <Button disabled variant="gradient"><Copy /> 复制</Button>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
