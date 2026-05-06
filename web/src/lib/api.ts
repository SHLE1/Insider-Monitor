import type { Config } from "@/types/config";
import type { TokenWatchResponse, WalletDataMap } from "@/types/wallet";

export async function fetchConfig(): Promise<Config> {
	const res = await fetch("/api/config");
	if (!res.ok) throw new Error(`加载配置失败: ${res.statusText}`);
	return res.json();
}

export async function saveConfig(config: Config): Promise<void> {
	const res = await fetch("/api/config", {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify(config),
	});
	if (!res.ok) {
		const msg = await res.text();
		throw new Error(msg || `保存失败: ${res.statusText}`);
	}
}

export async function fetchWalletData(): Promise<WalletDataMap> {
	const res = await fetch("/api/wallet-data");
	if (!res.ok) throw new Error(`加载钱包数据失败: ${res.statusText}`);
	return res.json();
}

export async function scanWallets(): Promise<WalletDataMap> {
	const res = await fetch("/api/scan", { method: "POST" });
	if (!res.ok) {
		const msg = await res.text();
		throw new Error(msg || `扫描失败: ${res.statusText}`);
	}
	return res.json();
}

export async function watchToken(params: {
	chain: string;
	wallet: string;
	token: string;
	limit?: number;
}): Promise<TokenWatchResponse> {
	const query = new URLSearchParams({
		chain: params.chain,
		wallet: params.wallet,
		token: params.token,
		limit: String(params.limit ?? 50),
	});
	const res = await fetch(`/api/token-watch?${query.toString()}`);
	if (!res.ok) {
		const msg = await res.text();
		throw new Error(msg || `读取 Token 交易失败: ${res.statusText}`);
	}
	return res.json();
}
