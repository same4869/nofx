import { useMemo, useState } from 'react'
import useSWR from 'swr'
import { api } from '../lib/api'
import { useLanguage } from '../contexts/LanguageContext'

interface TraderOrder {
  id?: number
  symbol?: string
  side?: string
  position_side?: string
  order_action?: string
  status?: string
  quantity?: number
  filled_quantity?: number
  avg_fill_price?: number
  price?: number
  commission?: number
  leverage?: number
  client_order_id?: string
  created_at?: string
  updated_at?: string
  filled_at?: string
}

interface OrderHistoryProps {
  traderId: string
}

function fmtPrice(v: number | undefined): string {
  if (!v || v === 0) return '-'
  if (v >= 1000) return v.toFixed(2)
  if (v >= 1) return v.toFixed(4)
  return v.toFixed(6)
}

function fmtNum(v: number | undefined, decimals = 4): string {
  if (v === undefined || v === null) return '-'
  return v.toFixed(decimals)
}

function colorForStatus(status: string | undefined): string {
  switch ((status || '').toUpperCase()) {
    case 'FILLED':
      return '#0ECB81'
    case 'NEW':
      return '#F0B90B'
    case 'CANCELED':
    case 'REJECTED':
    case 'EXPIRED':
      return '#F6465D'
    default:
      return '#848E9C'
  }
}

function colorForAction(action: string | undefined): string {
  const a = (action || '').toLowerCase()
  if (a.startsWith('open_')) return '#0ECB81'
  if (a.startsWith('close_')) return '#F0B90B'
  return '#848E9C'
}

export function OrderHistory({ traderId }: OrderHistoryProps) {
  const { language } = useLanguage()
  const [status, setStatus] = useState<string>('ALL')
  const [limit, setLimit] = useState<number>(100)

  const swrKey = useMemo(() => {
    if (!traderId) return null
    return ['orders', traderId, status, limit]
  }, [traderId, status, limit])

  const { data, isLoading, error, mutate } = useSWR<TraderOrder[]>(
    swrKey,
    async () => api.getOrders(traderId, { status: status === 'ALL' ? undefined : status, limit }),
    { refreshInterval: 5000 }
  )

  return (
    <div
      className="binance-card p-6"
      style={{ background: 'linear-gradient(135deg, #1E2329 0%, #181C21 100%)' }}
    >
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <span className="text-2xl">🧾</span>
          <h2 className="text-xl font-bold" style={{ color: '#EAECEF' }}>
            {language === 'zh' ? '订单记录' : 'Orders'}
          </h2>
          <span className="text-xs" style={{ color: '#848E9C' }}>
            {data ? `${data.length}` : ''}
          </span>
        </div>

        <div className="flex items-center gap-2">
          <select
            value={status}
            onChange={(e) => setStatus(e.target.value)}
            className="px-2 py-1 rounded text-xs"
            style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
          >
            <option value="ALL">{language === 'zh' ? '全部' : 'All'}</option>
            <option value="FILLED">FILLED</option>
            <option value="NEW">NEW</option>
            <option value="CANCELED">CANCELED</option>
          </select>

          <select
            value={limit}
            onChange={(e) => setLimit(parseInt(e.target.value) || 100)}
            className="px-2 py-1 rounded text-xs"
            style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
          >
            <option value={50}>50</option>
            <option value={100}>100</option>
            <option value={200}>200</option>
          </select>

          <button
            onClick={() => mutate()}
            className="px-2 py-1 rounded text-xs hover:opacity-80"
            style={{ background: '#2B3139', border: '1px solid #3C4043', color: '#EAECEF' }}
          >
            {language === 'zh' ? '刷新' : 'Refresh'}
          </button>
        </div>
      </div>

      {isLoading && (
        <div className="text-sm" style={{ color: '#848E9C' }}>
          {language === 'zh' ? '加载中…' : 'Loading…'}
        </div>
      )}
      {error && (
        <div className="text-sm" style={{ color: '#F6465D' }}>
          {(error as Error).message}
        </div>
      )}

      {data && data.length > 0 ? (
        <div className="overflow-x-auto">
          <table className="w-full text-xs">
            <thead className="text-left border-b" style={{ borderColor: '#2B3139', color: '#848E9C' }}>
              <tr>
                <th className="px-2 py-2 whitespace-nowrap">{language === 'zh' ? '时间' : 'Time'}</th>
                <th className="px-2 py-2 whitespace-nowrap">{language === 'zh' ? '币种' : 'Symbol'}</th>
                <th className="px-2 py-2 whitespace-nowrap">{language === 'zh' ? '动作' : 'Action'}</th>
                <th className="px-2 py-2 whitespace-nowrap text-center">{language === 'zh' ? '方向' : 'Side'}</th>
                <th className="px-2 py-2 whitespace-nowrap text-right">{language === 'zh' ? '数量' : 'Qty'}</th>
                <th className="px-2 py-2 whitespace-nowrap text-right">{language === 'zh' ? '成交价' : 'Fill'}</th>
                <th className="px-2 py-2 whitespace-nowrap text-center">{language === 'zh' ? '状态' : 'Status'}</th>
                <th className="px-2 py-2 whitespace-nowrap">{language === 'zh' ? '备注' : 'Note'}</th>
              </tr>
            </thead>
            <tbody>
              {data.map((o, idx) => {
                const when = o.filled_at || o.created_at || ''
                const timeStr = when ? new Date(when).toLocaleString() : '-'
                const sym = (o.symbol || '').replace('USDT', '')
                const act = o.order_action || '-'
                const st = o.status || '-'
                const note = (o.client_order_id || '').slice(0, 60)
                return (
                  <tr key={`${o.id ?? idx}-${idx}`} className="border-b" style={{ borderColor: '#1E2329' }}>
                    <td className="px-2 py-2 whitespace-nowrap" style={{ color: '#EAECEF' }}>{timeStr}</td>
                    <td className="px-2 py-2 font-mono whitespace-nowrap" style={{ color: '#EAECEF' }}>{sym}</td>
                    <td className="px-2 py-2 whitespace-nowrap" style={{ color: colorForAction(act) }}>{act}</td>
                    <td className="px-2 py-2 text-center whitespace-nowrap" style={{ color: '#EAECEF' }}>{(o.side || '').toUpperCase()}</td>
                    <td className="px-2 py-2 text-right font-mono whitespace-nowrap" style={{ color: '#EAECEF' }}>{fmtNum(o.quantity)}</td>
                    <td className="px-2 py-2 text-right font-mono whitespace-nowrap" style={{ color: '#EAECEF' }}>
                      {fmtPrice(o.avg_fill_price || o.price)}
                    </td>
                    <td className="px-2 py-2 text-center whitespace-nowrap" style={{ color: colorForStatus(st) }}>{st}</td>
                    <td className="px-2 py-2 text-[10px]" style={{ color: '#848E9C' }}>{note}</td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      ) : (
        !isLoading && (
          <div className="text-sm" style={{ color: '#848E9C' }}>
            {language === 'zh' ? '暂无订单' : 'No orders yet'}
          </div>
        )
      )}
    </div>
  )
}

