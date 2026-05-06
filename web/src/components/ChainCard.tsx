import { useState } from "react";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import type { ChainConfig, TokenConfig } from "@/types/config";
import { ChevronDown, ChevronUp, Trash2 } from "lucide-react";

interface Props {
	index: number;
	chain: ChainConfig;
	onChange: (c: ChainConfig) => void;
	onRemove: () => void;
}

function walletsToText(wallets: string[]): string {
	return (wallets ?? []).join("\n");
}

function textToWallets(text: string): string[] {
	return text
		.split("\n")
		.map((s) => s.trim())
		.filter(Boolean);
}

function tokensToText(tokens: TokenConfig[]): string {
	return (tokens ?? [])
		.map((t) => [t.address, t.symbol ?? "", t.decimals ?? ""].join(","))
		.join("\n");
}

function textToTokens(text: string): TokenConfig[] {
	return text
		.split("\n")
		.map((s) => s.trim())
		.filter(Boolean)
		.map((line) => {
			const [address, symbol, decimals] = line.split(",").map((v) => v.trim());
			return {
				address,
				symbol: symbol || undefined,
				decimals: decimals ? Number(decimals) : undefined,
			};
		});
}

function isConfigured(value?: string): boolean {
	return Boolean(value?.trim());
}

function SecretValueField({
	id,
	label,
	value,
	placeholder,
	onChange,
}: {
	id: string;
	label: string;
	value: string;
	placeholder: string;
	onChange: (value: string) => void;
}) {
	const [editing, setEditing] = useState(false);

	return (
		<div className="flex flex-col gap-1.5">
			<Label htmlFor={id} className="text-xs">
				{label}
			</Label>
			{editing ? (
				<div className="flex gap-2">
					<Input
						id={id}
						placeholder={placeholder}
						value={value}
						onChange={(e) => onChange(e.target.value)}
					/>
					<Button
						type="button"
						variant="outline"
						onClick={() => setEditing(false)}
					>
						完成
					</Button>
				</div>
			) : (
				<div className="flex items-center justify-between gap-3 rounded-md border px-3 py-2 text-sm">
					<span
						className={
							isConfigured(value) ? "text-green-600" : "text-muted-foreground"
						}
					>
						{isConfigured(value) ? "已配置" : "未配置"}
					</span>
					<Button
						type="button"
						variant="ghost"
						size="sm"
						onClick={() => setEditing(true)}
					>
						修改
					</Button>
				</div>
			)}
		</div>
	);
}

export function ChainCard({ index, chain, onChange, onRemove }: Props) {
	const [open, setOpen] = useState(true);

	const update = (patch: Partial<ChainConfig>) =>
		onChange({ ...chain, ...patch });

	const chainLabel = chain.name || `链 #${index + 1}`;
	const typeColor = chain.type === "solana" ? "secondary" : "default";

	return (
		<Card>
			<CardHeader className="pb-0">
				<div className="flex items-center justify-between gap-3">
					<div className="flex items-center gap-2 min-w-0">
						<button
							type="button"
							className="flex items-center gap-2 text-left min-w-0 hover:text-foreground"
							onClick={() => setOpen((o) => !o)}
						>
							{open ? (
								<ChevronUp className="size-4 shrink-0 text-muted-foreground" />
							) : (
								<ChevronDown className="size-4 shrink-0 text-muted-foreground" />
							)}
							<span className="font-semibold text-sm truncate">
								{chainLabel}
							</span>
						</button>
						<Badge variant={typeColor} className="text-xs shrink-0">
							{chain.type?.toUpperCase() ?? "EVM"}
						</Badge>
						{chain.rpc_url && (
							<span className="text-xs text-muted-foreground hidden sm:block">
								RPC 已配置
							</span>
						)}
					</div>
					<Button
						size="icon"
						variant="ghost"
						className="size-7 shrink-0 text-destructive hover:text-destructive"
						onClick={onRemove}
					>
						<Trash2 className="size-3.5" />
					</Button>
				</div>
			</CardHeader>

			{open && (
				<CardContent className="pt-4 flex flex-col gap-4">
					{/* Row 1: Name + Type */}
					<div className="grid grid-cols-2 gap-3">
						<div className="flex flex-col gap-1.5">
							<Label htmlFor={`name-${index}`} className="text-xs">
								链名称
							</Label>
							<Input
								id={`name-${index}`}
								value={chain.name}
								onChange={(e) => update({ name: e.target.value })}
							/>
						</div>
						<div className="flex flex-col gap-1.5">
							<Label htmlFor={`type-${index}`} className="text-xs">
								类型
							</Label>
							<Select
								value={chain.type}
								onValueChange={(v) => v && update({ type: v })}
							>
								<SelectTrigger id={`type-${index}`}>
									<SelectValue />
								</SelectTrigger>
								<SelectContent>
									<SelectItem value="evm">EVM</SelectItem>
									<SelectItem value="solana">Solana</SelectItem>
								</SelectContent>
							</Select>
						</div>
					</div>

					{/* RPC URL */}
					<SecretValueField
						id={`rpc-${index}`}
						label="RPC URL"
						placeholder="填写 RPC URL，或使用 .env 中的环境变量占位符"
						value={chain.rpc_url}
						onChange={(value) => update({ rpc_url: value })}
					/>

					{/* Row 2: Chain ID + Native Symbol (EVM only) */}
					{chain.type !== "solana" && (
						<div className="grid grid-cols-2 gap-3">
							<div className="flex flex-col gap-1.5">
								<Label htmlFor={`chainid-${index}`} className="text-xs">
									Chain ID
								</Label>
								<Input
									id={`chainid-${index}`}
									type="number"
									value={chain.chain_id ?? ""}
									onChange={(e) =>
										update({ chain_id: Number(e.target.value) || undefined })
									}
								/>
							</div>
							<div className="flex flex-col gap-1.5">
								<Label htmlFor={`symbol-${index}`} className="text-xs">
									原生币符号
								</Label>
								<Input
									id={`symbol-${index}`}
									placeholder="ETH / BNB / MATIC"
									value={chain.native_symbol ?? ""}
									onChange={(e) => update({ native_symbol: e.target.value })}
								/>
							</div>
						</div>
					)}

					<Separator />

					{/* Wallets */}
					<div className="flex flex-col gap-1.5">
						<div className="flex items-center justify-between">
							<Label className="text-xs">监控钱包地址</Label>
							<span className="text-xs text-muted-foreground">
								{(chain.wallets ?? []).length} 个
							</span>
						</div>
						<Textarea
							placeholder="每行一个地址"
							className="font-mono text-xs min-h-24 resize-y"
							value={walletsToText(chain.wallets)}
							onChange={(e) =>
								update({ wallets: textToWallets(e.target.value) })
							}
						/>
					</div>

					{/* Tokens */}
					<div className="flex flex-col gap-1.5">
						<div className="flex items-center justify-between">
							<Label className="text-xs">Token 合约白名单</Label>
							<span className="text-xs text-muted-foreground">
								{(chain.scan?.tokens ?? []).length} 个
							</span>
						</div>
						<Textarea
							placeholder={`每行一个：address,symbol,decimals\n例：0xabc...,USDT,6`}
							className="font-mono text-xs min-h-24 resize-y"
							value={tokensToText(chain.scan?.tokens ?? [])}
							onChange={(e) =>
								update({
									scan: {
										...chain.scan,
										scan_mode: "whitelist",
										tokens: textToTokens(e.target.value),
									},
								})
							}
						/>
					</div>
				</CardContent>
			)}
		</Card>
	);
}
