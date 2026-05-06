export interface TokenAccountInfo {
	balance: number;
	raw_balance?: string;
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
