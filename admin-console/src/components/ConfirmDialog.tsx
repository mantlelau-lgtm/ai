import { Modal } from '@/components/Modal'
import { MiniButton, SecondaryButton } from '@/components/ConfigControls'

type ConfirmDialogProps = {
  open: boolean
  title: string
  description?: string
  confirmText?: string
  cancelText?: string
  busy?: boolean
  tone?: 'danger' | 'primary'
  onConfirm: () => void | Promise<void>
  onCancel: () => void
}

export function ConfirmDialog({
  open,
  title,
  description,
  confirmText = '确认',
  cancelText = '取消',
  busy,
  tone = 'primary',
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  return (
    <Modal
      open={open}
      title={title}
      description={description}
      eyebrow={tone === 'danger' ? 'Confirm' : 'Notice'}
      onClose={() => {
        if (!busy) {
          onCancel()
        }
      }}
      footer={
        <>
          <SecondaryButton onClick={onCancel} disabled={busy}>
            {cancelText}
          </SecondaryButton>
          {tone === 'danger' ? (
            <button
              type="button"
              disabled={busy}
              onClick={() => void onConfirm()}
              className="inline-flex items-center gap-2 rounded-2xl border border-rose-300/40 bg-rose-500/90 px-4 py-3 text-sm font-medium text-white transition hover:bg-rose-400 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {busy ? '处理中...' : confirmText}
            </button>
          ) : (
            <MiniButton disabled={busy} onClick={() => void onConfirm()}>
              {busy ? '处理中...' : confirmText}
            </MiniButton>
          )}
        </>
      }
    >
      <div className="text-sm leading-6 text-slate-200">
        {description ? null : '该操作不可撤销，请确认是否继续。'}
      </div>
    </Modal>
  )
}
