import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { FilePlus2, LogIn, PanelLeftOpen, Sparkles, UserPlus, Wand2, X } from 'lucide-react'
import { useDocsStore } from '@/store/docs'
import { useAIChatStore } from '@/store/aiChat'
import { useAuthStore } from '@/store/auth'
import { Shares, type DocNode } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { DocTree } from '@/components/DocTree'
import { DocViewer } from '@/components/DocViewer'
import { CreateDocDialog } from '@/components/CreateDocDialog'
import { ShareDialog } from '@/components/ShareDialog'
import { AISettingsDialog } from '@/components/AISettingsDialog'
import { AuthDialog } from '@/components/AuthDialog'
import { UserMenu } from '@/components/UserMenu'

export default function HomePage() {
  const { nodes, loadAll, selectedId, sidebarOpen, toggleSidebar, selectDoc, createNode, upsertFromServer } = useDocsStore()
  const { openPanel } = useAIChatStore()
  const { user, bootstrapped, bootstrap, openLogin } = useAuthStore()
  const [createOpen, setCreateOpen] = useState(false)
  const [createParent, setCreateParent] = useState<string | null>(null)
  const [shareDoc, setShareDoc] = useState<DocNode | null>(null)
  const [aiSettingsOpen, setAISettingsOpen] = useState(false)

  const { docId: routeDocId } = useParams<{ docId?: string }>()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const fullscreen = searchParams.get('fullscreen') !== null
    && searchParams.get('fullscreen') !== '0'
    && searchParams.get('fullscreen') !== 'false'
  const shareToken = searchParams.get('share')

  // 节点全量列表需要登录（后端 GET /api/nodes 挂了 AuthRequired）；
  // 未登录访客（分享链接场景）不拉列表，靠 SharePage 的 upsertFromServer 注入单个文档。
  useEffect(() => { if (user) loadAll() }, [user, loadAll])
  useEffect(() => { bootstrap() }, [bootstrap])

  // URL 带 ?share=<token>（SharePage 跳转时附带）：刷新后 store 已清空，
  // 凭 token 重新解析分享并恢复文档，让访客刷新后仍可匿名浏览。
  const [shareRestorePending, setShareRestorePending] = useState(!!shareToken)
  const restoredShareToken = useRef<string | null>(null)
  useEffect(() => {
    if (!shareToken || restoredShareToken.current === shareToken) return
    restoredShareToken.current = shareToken
    // 刚从 SharePage 跳转过来时文档已在 store 里，无需重复请求
    if (routeDocId && useDocsStore.getState().sharedDocIds.includes(routeDocId)) {
      setShareRestorePending(false)
      return
    }
    let cancelled = false
    Shares.info(shareToken)
      .then((r) => { if (!cancelled) upsertFromServer(r.doc, { shared: true }) })
      .catch((err) => {
        console.warn('[web-doc share] restore from ?share= failed', {
          shareToken,
          status: err?.response?.status,
          message: err?.message,
        })
      })
      .finally(() => { if (!cancelled) setShareRestorePending(false) })
    return () => { cancelled = true }
  }, [shareToken, routeDocId, upsertFromServer])

  // 登录态校验完成后仍未登录：自动弹出登录/注册框（每次进入页面只主动弹一次）。
  // 分享访客除外——他们靠分享 token 恢复的文档匿名浏览，不应被打扰。
  const autoPromptedLogin = useRef(false)
  useEffect(() => {
    if (!bootstrapped || shareRestorePending || autoPromptedLogin.current) return
    autoPromptedLogin.current = true
    if (user) return
    const { sharedDocIds } = useDocsStore.getState()
    if (routeDocId && sharedDocIds.includes(routeDocId)) return
    openLogin('login')
  }, [bootstrapped, shareRestorePending, user, routeDocId, openLogin])

  // URL → store
  useEffect(() => {
    console.debug('[web-doc route] URL -> store', {
      pathname: location.pathname,
      search: location.search,
      routeDocId,
      selectedId,
    })
    if (routeDocId && routeDocId !== selectedId) {
      console.debug('[web-doc route] select doc from URL', { routeDocId, selectedId })
      selectDoc(routeDocId)
    }
    if (!routeDocId && selectedId) {
      console.debug('[web-doc route] clear selected doc because URL has no doc id', { selectedId })
      selectDoc(null)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [routeDocId])

  // store → URL（保持 query 参数，例如 fullscreen）
  useEffect(() => {
    if (routeDocId && !selectedId) {
      console.debug('[web-doc route] skip store -> URL while URL doc is being synced to store', {
        routeDocId,
        selectedId,
        pathname: location.pathname,
        search: location.search,
      })
      return
    }

    const search = location.search || ''
    const target = (selectedId ? `/v/${selectedId}` : '/') + search
    const current = (routeDocId ? `/v/${routeDocId}` : '/') + search
    if (target !== current) {
      console.debug('[web-doc route] store -> URL navigate', {
        selectedId,
        routeDocId,
        current,
        target,
      })
      navigate(target, { replace: false })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedId])

  const selectedDoc = useMemo<DocNode | null>(
    () => nodes.find((n) => n.id === selectedId && n.type === 'doc') ?? null,
    [nodes, selectedId],
  )

  // URL 指向不存在文档：清空（仅登录用户、非 fullscreen 模式才校验，
  // 避免分享访客/未登录用户因本地 nodes 列表不包含被分享文档而被误重定向回首页）。
  useEffect(() => {
    if (!routeDocId || nodes.length === 0) return
    if (!user) {
      console.debug('[web-doc route] skip missing-doc validation for anonymous visitor', {
        routeDocId,
        nodesCount: nodes.length,
        fullscreen,
      })
      return
    }
    if (fullscreen) {
      console.debug('[web-doc route] skip missing-doc validation in fullscreen mode', {
        routeDocId,
        nodesCount: nodes.length,
        userId: user.id,
        username: user.username,
      })
      return
    }
    // ?share= 恢复尚未完成时不校验，避免把正在恢复的分享文档误判为「不存在」
    if (shareRestorePending) {
      console.debug('[web-doc route] skip missing-doc validation while share context restoring', {
        routeDocId,
        shareToken,
      })
      return
    }
    const exists = nodes.some((n) => n.id === routeDocId && n.type === 'doc')
    console.debug('[web-doc route] validate route doc against node list', {
      routeDocId,
      exists,
      nodesCount: nodes.length,
      userId: user.id,
      username: user.username,
      visibleDocIds: nodes.filter((n) => n.type === 'doc').map((n) => n.id),
    })
    if (!exists) {
      console.warn('[web-doc route] route doc is missing from current node list, navigate to home', {
        routeDocId,
        nodesCount: nodes.length,
        userId: user.id,
        username: user.username,
      })
      selectDoc(null)
      navigate('/', { replace: true })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [nodes, routeDocId, user, shareRestorePending])

  const handleCreate = (parentId: string | null) => {
    if (!user) {
      openLogin('login')
      return
    }
    setCreateParent(parentId)
    setCreateOpen(true)
  }

  // 入口："AI 生成"：创建一个占位空文档，进入文档详情并自动打开 AI 面板
  const handleStartAI = async (parentId: string | null) => {
    if (!user) {
      openLogin('login')
      return
    }
    const node = await createNode({
      parentId,
      type: 'doc',
      title: 'AI 新文档',
    })
    selectDoc(node.id)
    openPanel()
  }

  // ========== Fullscreen 模式：仅显示文档纯净预览（无外壳） ==========
  if (fullscreen) {
    return (
      <div className="relative h-full w-full overflow-hidden bg-background">
        {selectedDoc ? (
          <DocViewer
            doc={selectedDoc}
            onShare={setShareDoc}
            onOpenAISettings={() => setAISettingsOpen(true)}
            chromeless
          />
        ) : (
          <div className="h-full w-full flex items-center justify-center text-sm text-muted-foreground">
            {bootstrapped && !user && !shareRestorePending ? '该文档需要登录后查看' : '加载中…'}
          </div>
        )}
        <ShareDialog doc={shareDoc} open={!!shareDoc} onOpenChange={(v) => !v && setShareDoc(null)} />
        <AuthDialog />
      </div>
    )
  }

  return (
    <div className="relative h-full w-full overflow-hidden bg-background flex">
      {/* 侧栏：通过 sidebarOpen 控制显隐 */}
      {sidebarOpen && (
        <aside className="h-full w-72 shrink-0 bg-card border-r border-border/60 shadow-xl flex flex-col">
          <SidebarHeader
            onToggle={toggleSidebar}
            onNew={() => handleCreate(null)}
            onAISettings={() => { if (!user) { openLogin('login'); return } setAISettingsOpen(true) }}
          />
          <div className="flex-1 overflow-y-auto py-2">
            <DocTree onCreateInFolder={handleCreate} />
          </div>
          <SidebarFooter count={nodes.filter((n) => n.type === 'doc').length} />
        </aside>
      )}

      {/* 主预览区域 */}
      <main className="relative flex-1 min-w-0 h-full">
        {selectedDoc ? (
          <DocViewer
            doc={selectedDoc}
            onShare={setShareDoc}
            onOpenAISettings={() => setAISettingsOpen(true)}
            sidebarOpen={sidebarOpen}
            onToggleSidebar={toggleSidebar}
          />
        ) : (
          <EmptyState
            sidebarOpen={sidebarOpen}
            onToggleSidebar={toggleSidebar}
            onCreate={() => handleCreate(null)}
            onAI={() => handleStartAI(null)}
          />
        )}
      </main>

      {/* 弹窗 */}
      <CreateDocDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        parentId={createParent}
        onAITrigger={(pid) => { setCreateOpen(false); setTimeout(() => handleStartAI(pid), 150) }}
      />
      <ShareDialog doc={shareDoc} open={!!shareDoc} onOpenChange={(v) => !v && setShareDoc(null)} />
      <AISettingsDialog open={aiSettingsOpen} onOpenChange={setAISettingsOpen} />
      <AuthDialog />
    </div>
  )
}

function SidebarHeader({
  onToggle, onNew, onAISettings,
}: {
  onToggle: () => void
  onNew: () => void
  onAISettings: () => void
}) {
  return (
    <div className="flex items-center gap-2 px-3 py-3 border-b border-border/60">
      <div className="flex items-center gap-2 flex-1 min-w-0">
        <div className="h-7 w-7 rounded-md bg-gradient-to-br from-blue-500 to-violet-500 flex items-center justify-center shadow-md">
          <Sparkles className="h-4 w-4 text-white" />
        </div>
        <div className="min-w-0">
          <div className="text-sm font-semibold text-gradient leading-none">Web-Doc</div>
          <div className="text-[10px] text-muted-foreground mt-0.5">HTML 文档站</div>
        </div>
      </div>
      <Tooltip>
        <TooltipTrigger asChild>
          <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onAISettings}>
            <Wand2 className="text-violet-400" />
          </Button>
        </TooltipTrigger>
        <TooltipContent>AI 设置</TooltipContent>
      </Tooltip>
      <Tooltip>
        <TooltipTrigger asChild>
          <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onNew}>
            <FilePlus2 />
          </Button>
        </TooltipTrigger>
        <TooltipContent>新建</TooltipContent>
      </Tooltip>
      <Tooltip>
        <TooltipTrigger asChild>
          <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onToggle}>
            <X />
          </Button>
        </TooltipTrigger>
        <TooltipContent>关闭侧栏</TooltipContent>
      </Tooltip>
    </div>
  )
}

function SidebarFooter({ count }: { count: number }) {
  return (
    <div className="px-3 py-2 border-t border-border/60 text-[11px] text-muted-foreground flex items-center justify-between gap-2">
      <span className="truncate">共 {count} 个文档</span>
      <UserMenu />
    </div>
  )
}

function EmptyState({
  sidebarOpen, onToggleSidebar, onCreate, onAI,
}: {
  sidebarOpen: boolean
  onToggleSidebar: () => void
  onCreate: () => void
  onAI: () => void
}) {
  const { user, bootstrapped, openLogin, registerEnabled } = useAuthStore()
  const anonymous = bootstrapped && !user
  return (
    <div className="relative h-full w-full flex items-center justify-center gradient-bg">
      {/* 顶部仅在侧栏关闭时显示打开按钮 */}
      {!sidebarOpen && (
        <div className="absolute left-0 top-0 z-10 px-3 py-2">
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="ghost" size="icon" onClick={onToggleSidebar}>
                <PanelLeftOpen />
              </Button>
            </TooltipTrigger>
            <TooltipContent>打开侧栏</TooltipContent>
          </Tooltip>
        </div>
      )}
      <div className="text-center max-w-md px-8">
        <div className="mx-auto h-16 w-16 rounded-2xl bg-gradient-to-br from-blue-500 to-violet-500 flex items-center justify-center shadow-xl shadow-violet-500/30 mb-6">
          <Sparkles className="h-8 w-8 text-white" />
        </div>
        <h1 className="text-3xl font-bold tracking-tight mb-3">
          欢迎使用 <span className="text-gradient">Web-Doc</span>
        </h1>
        <p className="text-muted-foreground mb-8 leading-relaxed">
          像管理 Markdown 一样管理 AI 生成的 HTML 文档。<br />
          沙箱预览、文件夹热更新、一键分享。
        </p>
        <div className="flex items-center justify-center gap-3">
          {anonymous ? (
            <>
              <Button variant="gradient" size="lg" onClick={() => openLogin('login')}>
                <LogIn /> 登录
              </Button>
              {registerEnabled && (
                <Button variant="outline" size="lg" onClick={() => openLogin('register')}>
                  <UserPlus /> 注册账号
                </Button>
              )}
            </>
          ) : (
            <>
              <Button variant="gradient" size="lg" onClick={onAI}>
                <Sparkles /> AI 生成文档
              </Button>
              <Button variant="outline" size="lg" onClick={onCreate}>
                <FilePlus2 /> 手动创建
              </Button>
            </>
          )}
        </div>
        {anonymous && (
          <p className="mt-4 text-xs text-muted-foreground">
            登录后即可创建、编辑与分享文档
          </p>
        )}
      </div>
    </div>
  )
}
