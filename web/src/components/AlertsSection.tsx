import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import type { Config } from '@/types/config'
import { Bell } from 'lucide-react'

interface Props {
  draft: Config
  onChange: (c: Config) => void
}

export function AlertsSection({ draft, onChange }: Props) {
  const alerts = draft.alerts ?? { minimum_balance: 1000, significant_change: 20 }

  const update = (patch: Partial<typeof alerts>) =>
    onChange({ ...draft, alerts: { ...alerts, ...patch } })

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm flex items-center gap-2">
          <Bell className="size-3.5 text-muted-foreground" />
          提醒阈值
        </CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="minBalance" className="text-xs">最低余额（USD）</Label>
          <Input
            id="minBalance"
            type="number"
            min={0}
            step={1}
            value={alerts.minimum_balance}
            onChange={e => update({ minimum_balance: Number(e.target.value) })}
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="sigChange" className="text-xs">明显变化比例（%）</Label>
          <Input
            id="sigChange"
            type="number"
            min={0}
            step={0.1}
            value={alerts.significant_change}
            onChange={e => update({ significant_change: Number(e.target.value) })}
          />
          <p className="text-xs text-muted-foreground">超过此百分比时触发提醒</p>
        </div>
      </CardContent>
    </Card>
  )
}
