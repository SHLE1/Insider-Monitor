export interface TokenConfig {
  address: string
  symbol?: string
  decimals?: number
}

export interface ScanConfig {
  scan_mode: string
  tokens: TokenConfig[]
  include_tokens?: string[]
  exclude_tokens?: string[]
}

export interface ChainConfig {
  type: string
  name: string
  rpc_url: string
  chain_id?: number
  native_symbol?: string
  wallets: string[]
  scan: ScanConfig
}

export interface AlertConfig {
  minimum_balance: number
  significant_change: number
  ignore_tokens?: string[]
}

export interface DiscordConfig {
  enabled: boolean
  webhook_url: string
  channel_id?: string
}

export interface TelegramConfig {
  enabled: boolean
  bot_token: string
  chat_id: string
}

export interface Config {
  scan_interval: string
  alerts: AlertConfig
  discord: DiscordConfig
  telegram: TelegramConfig
  chains: ChainConfig[]
}
