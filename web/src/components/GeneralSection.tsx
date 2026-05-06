import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import type { Config } from '@/types/config'
import { Clock } from 'lucide-react'

interface Props {
  draft: Config
  onChange: (c: Config) => void
}

export function GeneralSection({ draft, onChange }: Props) {
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm flex items-center gap-2">
          <Clock className="size-3.5 text-muted-foreground" />
          通用设置
        </CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="scanInterval" className="text-xs">扫描间隔</Label>
          <Input
            id="scanInterval"
            placeholder="1m"
            value={draft.scan_interval ?? '1m'}
            onChange={e => onChange({ ...draft, scan_interval: e.target.value })}
          />
          <p className="text-xs text-muted-foreground">支持 30s、1m、5m 等格式</p>
        </div>
      </CardContent>
    </Card>
  )
}
