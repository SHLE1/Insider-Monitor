export interface TokenAccountInfo {
	balance: number;
	raw_balance?: string;
	total_supply_raw?: string;
	holding_percent?: number;
	last_updated: string;
	symbol: string;
	decimals: number;
	configured?: boolean;
	usd_price: number;
	usd_value: number;
	confidence_level: string;
}

export interface WalletData {
	chain_name: string;
	chain_type: string;
	wallet_address: string;
	token_accounts: Record<string, TokenAccountInfo>;
	last_scanned: string;
}

export type WalletDataMap = Record<string, WalletData>;

export interface TokenTransfer {
	tx_hash: string;
	from: string;
	to: string;
	amount: number;
	raw_amount: string;
	direction: string;
	block_number: number;
	log_index: number;
}

export interface TokenAlert {
	ChainName: string;
	WalletAddress: string;
	TokenMint: string;
	TokenSymbol: string;
	ChangeType: string;
	OldBalance: number;
	NewBalance: number;
	ChangePercent: number;
	AmountChanged: number;
	Direction: string;
	OldHoldingPct: number;
	NewHoldingPct: number;
	TxHash?: string;
	FromAddress?: string;
	ToAddress?: string;
}

export interface TokenWatchResponse {
	wallet_data: WalletDataMap;
	transfers: TokenTransfer[];
	alerts: TokenAlert[];
}
