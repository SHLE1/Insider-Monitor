import { useState, useEffect } from "react";
import { Toaster } from "@/components/ui/sonner";
import { toast } from "sonner";
import {
	fetchConfig,
	fetchWalletData,
	saveConfig,
	scanWallets,
} from "@/lib/api";
import type { Config, ChainConfig } from "@/types/config";
import type { WalletData, WalletDataMap } from "@/types/wallet";
import { GeneralSection } from "@/components/GeneralSection";
import { AlertsSection } from "@/components/AlertsSection";
import { NotificationsSection } from "@/components/NotificationsSection";
import { ChainCard } from "@/components/ChainCard";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import {
	PlusCircle,
	RotateCcw,
	Save,
	Activity,
	Settings,
	WalletCards,
	Link,
} from "lucide-react";

const DEFAULT_CHAIN: ChainConfig = {
	type: "evm",
	name: "BSC",
	rpc_url: "",
	chain_id: 56,
	native_symbol: "BNB",
	wallets: [],
	scan: { scan_mode: "whitelist", tokens: [] },
};

type Page = "dashboard" | "settings";

function formatAmount(
	balance: number,
	decimals: number,
	rawBalance?: string,
): string {
	const source =
		rawBalance && /^\d+$/.test(rawBalance) ? rawBalance : undefined;
	const value = source
		? Number.parseFloat(
				`${source.slice(0, -decimals) || "0"}.${source.slice(-decimals).padStart(decimals, "0")}`,
			)
		: balance / 10 ** decimals;
	return new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 6 }).format(
		value,
	);
}

function formatUSD(value: number): string {
	return new Intl.NumberFormat("zh-CN", {
		style: "currency",
		currency: "USD",
		maximumFractionDigits: 2,
	}).format(value || 0);
}

function formatTime(value?: string): string {
	if (!value) return "暂无扫描记录";
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) return "暂无扫描记录";
	return date.toLocaleString("zh-CN");
}

function walletKey(chainName: string, wallet: string): string {
	return chainName ? `${chainName}:${wallet}` : wallet;
}

function findWalletData(
	data: WalletDataMap,
	chainName: string,
	wallet: string,
): WalletData | undefined {
	return (
		data[walletKey(chainName, wallet)] ??
		Object.values(data).find(
			(item) => item.chain_name === chainName && item.wallet_address === wallet,
		)
	);
}

export default function App() {
	const [config, setConfig] = useState<Config | null>(null);
	const [draft, setDraft] = useState<Config | null>(null);
	const [walletData, setWalletData] = useState<WalletDataMap>({});
	const [loading, setLoading] = useState(true);
	const [saving, setSaving] = useState(false);
	const [scanning, setScanning] = useState(false);
	const [page, setPage] = useState<Page>("dashboard");

	useEffect(() => {
		const loadData = async () => {
			try {
				const [configData, walletDataMap] = await Promise.all([
					fetchConfig(),
					fetchWalletData(),
				]);
				setConfig(configData);
				setDraft(JSON.parse(JSON.stringify(configData)));
				setWalletData(walletDataMap);
			} catch (e) {
				toast.error(String(e));
			} finally {
				setLoading(false);
			}
		};

		void loadData();
	}, []);

	const reset = () => {
		if (config) setDraft(JSON.parse(JSON.stringify(config)));
	};

	const save = async () => {
		if (!draft) return;
		setSaving(true);
		try {
			await saveConfig(draft);
			setConfig(JSON.parse(JSON.stringify(draft)));
			toast.success("配置已保存");
		} catch (e) {
			toast.error(String(e));
		} finally {
			setSaving(false);
		}
	};

	const scanNow = async () => {
		setScanning(true);
		try {
			const data = await scanWallets();
			setWalletData(data);
			toast.success("扫描完成，持仓数据已更新");
		} catch (e) {
			toast.error(String(e));
		} finally {
			setScanning(false);
		}
	};

	const addChain = () => {
		if (!draft) return;
		setDraft({
			...draft,
			chains: [...(draft.chains ?? []), { ...DEFAULT_CHAIN }],
		});
	};

	const removeChain = (i: number) => {
		if (!draft) return;
		const chains = [...draft.chains];
		chains.splice(i, 1);
		setDraft({ ...draft, chains });
	};

	const updateChain = (i: number, chain: ChainConfig) => {
		if (!draft) return;
		const chains = [...draft.chains];
		chains[i] = chain;
		setDraft({ ...draft, chains });
	};

	if (loading) {
		return (
			<div className="min-h-screen flex items-center justify-center bg-background">
				<div className="flex items-center gap-3 text-muted-foreground">
					<Activity className="size-5 animate-pulse" />
					<span>正在加载监控信息…</span>
				</div>
			</div>
		);
	}

	if (!draft) return null;

	const chains = draft.chains ?? [];
	const walletCount = chains.reduce(
		(total, chain) => total + (chain.wallets?.length ?? 0),
		0,
	);
	const holdings = Object.values(walletData).flatMap((wallet) =>
		Object.entries(wallet.token_accounts ?? {}).map(([mint, token]) => ({
			wallet,
			mint,
			token,
		})),
	);
	const tokenCount = holdings.length;
	const totalUSD = holdings.reduce(
		(total, item) => total + (item.token.usd_value || 0),
		0,
	);
	const notificationCount =
		Number(Boolean(draft.discord?.enabled)) +
		Number(Boolean(draft.telegram?.enabled));

	return (
		<div className="min-h-screen bg-background">
			<Toaster richColors position="top-right" />

			<header className="border-b bg-card sticky top-0 z-10">
				<div className="max-w-6xl mx-auto px-6 py-4 flex items-center justify-between gap-4">
					<div className="flex items-center gap-3">
						<div className="size-8 rounded-lg bg-primary flex items-center justify-center">
							<Activity className="size-4 text-primary-foreground" />
						</div>
						<div>
							<h1 className="text-lg font-semibold leading-tight">
								Insider Monitor
							</h1>
							<p className="text-xs text-muted-foreground">
								钱包交易与持仓监控
							</p>
						</div>
					</div>
					<div className="flex items-center gap-2">
						<Button
							variant={page === "dashboard" ? "default" : "outline"}
							size="sm"
							onClick={() => setPage("dashboard")}
							className="gap-1.5"
						>
							<WalletCards className="size-3.5" />
							主界面
						</Button>
						<Button
							variant={page === "settings" ? "default" : "outline"}
							size="sm"
							onClick={() => setPage("settings")}
							className="gap-1.5"
						>
							<Settings className="size-3.5" />
							配置
						</Button>
						{page === "settings" && (
							<>
								<Button
									variant="outline"
									size="sm"
									onClick={reset}
									className="gap-1.5"
								>
									<RotateCcw className="size-3.5" />
									重置
								</Button>
								<Button
									size="sm"
									onClick={save}
									disabled={saving}
									className="gap-1.5"
								>
									<Save className="size-3.5" />
									{saving ? "保存中…" : "保存配置"}
								</Button>
							</>
						)}
					</div>
				</div>
			</header>

			<main className="max-w-6xl mx-auto px-6 py-8">
				{page === "dashboard" ? (
					<section className="flex flex-col gap-6">
						<div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
							<div>
								<h2 className="text-base font-semibold">监控概览</h2>
								<p className="text-sm text-muted-foreground mt-1">
									主界面展示当前持仓、最近扫描时间和历史扫描结果；RPC、Token、通知密钥等请到“配置”页维护。
								</p>
							</div>
							<Button onClick={scanNow} disabled={scanning} className="gap-1.5">
								<Activity
									className={scanning ? "size-3.5 animate-spin" : "size-3.5"}
								/>
								{scanning ? "扫描中…" : "立即扫描"}
							</Button>
						</div>

						<div className="grid grid-cols-1 md:grid-cols-4 gap-4">
							<Card>
								<CardHeader className="pb-2">
									<CardTitle className="text-sm">监控地址</CardTitle>
								</CardHeader>
								<CardContent className="text-2xl font-semibold">
									{walletCount}
								</CardContent>
							</Card>
							<Card>
								<CardHeader className="pb-2">
									<CardTitle className="text-sm">当前持仓</CardTitle>
								</CardHeader>
								<CardContent className="text-2xl font-semibold">
									{tokenCount}
								</CardContent>
							</Card>
							<Card>
								<CardHeader className="pb-2">
									<CardTitle className="text-sm">估算总价值</CardTitle>
								</CardHeader>
								<CardContent className="text-2xl font-semibold">
									{formatUSD(totalUSD)}
								</CardContent>
							</Card>
							<Card>
								<CardHeader className="pb-2">
									<CardTitle className="text-sm">通知渠道</CardTitle>
								</CardHeader>
								<CardContent className="text-2xl font-semibold">
									{notificationCount}
								</CardContent>
							</Card>
						</div>

						<div className="flex flex-col gap-4">
							{chains.length === 0 ? (
								<div className="rounded-lg border border-dashed p-12 flex flex-col items-center justify-center gap-3 text-muted-foreground">
									<Activity className="size-8 opacity-30" />
									<p className="text-sm">
										暂无监控链，请到“配置”页添加链和地址。
									</p>
								</div>
							) : (
								chains.map((chain, i) => (
									<Card key={i}>
										<CardHeader>
											<CardTitle className="text-sm flex items-center justify-between gap-3">
												<span>{chain.name || `链 #${i + 1}`}</span>
												<span className="text-xs text-muted-foreground font-normal">
													{chain.type?.toUpperCase() ?? "EVM"}
												</span>
											</CardTitle>
										</CardHeader>
										<CardContent className="flex flex-col gap-5">
											{(chain.wallets ?? []).length === 0 ? (
												<p className="text-sm text-muted-foreground">
													暂无地址
												</p>
											) : (
												(chain.wallets ?? []).map((wallet, walletIndex) => {
													const current = findWalletData(
														walletData,
														chain.name,
														wallet,
													);
													const tokens = Object.entries(
														current?.token_accounts ?? {},
													);
													const walletUSD = tokens.reduce(
														(total, [, token]) =>
															total + (token.usd_value || 0),
														0,
													);
													return (
														<div
															key={`${wallet}-${walletIndex}`}
															className="rounded-lg border p-4 flex flex-col gap-4"
														>
															<div className="flex flex-col gap-2 md:flex-row md:items-start md:justify-between">
																<div>
																	<div className="text-xs text-muted-foreground mb-1">
																		监控地址
																	</div>
																	<div className="font-mono text-xs break-all">
																		{wallet}
																	</div>
																</div>
																<div className="text-xs text-muted-foreground md:text-right">
																	最近扫描：{formatTime(current?.last_scanned)}
																</div>
															</div>
															<div className="grid grid-cols-1 md:grid-cols-3 gap-3 text-sm">
																<div className="rounded-md bg-muted p-3">
																	<div className="text-xs text-muted-foreground">
																		当前持仓
																	</div>
																	<div className="font-semibold mt-1">
																		{tokens.length} 个 Token
																	</div>
																</div>
																<div className="rounded-md bg-muted p-3">
																	<div className="text-xs text-muted-foreground">
																		估算价值
																	</div>
																	<div className="font-semibold mt-1">
																		{formatUSD(walletUSD)}
																	</div>
																</div>
																<div className="rounded-md bg-muted p-3">
																	<div className="text-xs text-muted-foreground">
																		历史交易 / 变化
																	</div>
																	<div className="font-semibold mt-1">
																		基于最近两次扫描生成提醒
																	</div>
																</div>
															</div>
															<Separator />
															<div className="flex flex-col gap-2">
																<div className="text-xs font-semibold text-muted-foreground">
																	当前持仓明细
																</div>
																{tokens.length === 0 ? (
																	<p className="text-sm text-muted-foreground">
																		暂无持仓数据。请先运行监控扫描，扫描结果会写入
																		data/wallet_data.json 后显示在这里。
																	</p>
																) : (
																	tokens.map(([mint, token]) => (
																		<div
																			key={mint}
																			className="grid grid-cols-1 md:grid-cols-[1fr_auto_auto] gap-2 rounded-md border px-3 py-2 text-sm"
																		>
																			<div>
																				<div className="font-medium">
																					{token.symbol || "UNKNOWN"}
																				</div>
																				<div className="font-mono text-xs text-muted-foreground break-all">
																					{mint}
																				</div>
																			</div>
																			<div className="font-mono md:text-right">
																				{formatAmount(
																					token.balance,
																					token.decimals,
																					token.raw_balance,
																				)}
																			</div>
																			<div className="md:text-right text-muted-foreground">
																				{formatUSD(token.usd_value)}
																			</div>
																		</div>
																	))
																)}
															</div>
														</div>
													);
												})
											)}
										</CardContent>
									</Card>
								))
							)}
						</div>
					</section>
				) : (
					<div className="flex gap-6 items-start">
						<aside className="w-72 shrink-0 flex flex-col gap-4">
							<GeneralSection draft={draft} onChange={setDraft} />
							<AlertsSection draft={draft} onChange={setDraft} />
							<NotificationsSection draft={draft} onChange={setDraft} />
						</aside>
						<section className="flex-1 min-w-0 flex flex-col gap-4">
							<div className="flex items-center justify-between">
								<div>
									<h2 className="text-sm font-semibold flex items-center gap-2">
										<Link className="size-3.5" />
										链配置
									</h2>
									<p className="text-xs text-muted-foreground mt-0.5">
										共 {chains.length} 条链。RPC
										与通知密钥可填写真实值，也可填写 .env 中的环境变量占位符。
									</p>
								</div>
								<Button
									size="sm"
									variant="outline"
									onClick={addChain}
									className="gap-1.5"
								>
									<PlusCircle className="size-3.5" />
									添加链
								</Button>
							</div>
							<Separator />
							{chains.length === 0 ? (
								<div className="rounded-lg border border-dashed p-12 flex flex-col items-center justify-center gap-3 text-muted-foreground">
									<Activity className="size-8 opacity-30" />
									<p className="text-sm">暂无链配置，点击右上角“添加链”开始</p>
								</div>
							) : (
								<div className="flex flex-col gap-4">
									{chains.map((chain, i) => (
										<ChainCard
											key={i}
											index={i}
											chain={chain}
											onChange={(c) => updateChain(i, c)}
											onRemove={() => removeChain(i)}
										/>
									))}
								</div>
							)}
						</section>
					</div>
				)}
			</main>
		</div>
	);
}
