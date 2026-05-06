import { useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Separator } from "@/components/ui/separator";
import { Button } from "@/components/ui/button";
import type { Config } from "@/types/config";
import { MessageSquare } from "lucide-react";

interface Props {
	draft: Config;
	onChange: (c: Config) => void;
}

function isConfigured(value?: string): boolean {
	return Boolean(value?.trim());
}

function SecretField({
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

export function NotificationsSection({ draft, onChange }: Props) {
	const discord = draft.discord ?? { enabled: false, webhook_url: "" };
	const telegram = draft.telegram ?? {
		enabled: false,
		bot_token: "",
		chat_id: "",
	};

	const updateDiscord = (patch: Partial<typeof discord>) =>
		onChange({ ...draft, discord: { ...discord, ...patch } });

	const updateTelegram = (patch: Partial<typeof telegram>) =>
		onChange({ ...draft, telegram: { ...telegram, ...patch } });

	return (
		<Card>
			<CardHeader className="pb-3">
				<CardTitle className="text-sm flex items-center gap-2">
					<MessageSquare className="size-3.5 text-muted-foreground" />
					通知渠道
				</CardTitle>
			</CardHeader>
			<CardContent className="flex flex-col gap-4">
				{/* Discord */}
				<div className="flex flex-col gap-3">
					<div className="flex items-center justify-between">
						<Label className="text-xs font-semibold">Discord</Label>
						<Switch
							checked={discord.enabled}
							onCheckedChange={(v) => updateDiscord({ enabled: v })}
						/>
					</div>
					{discord.enabled && (
						<SecretField
							id="discordWebhook"
							label="Webhook URL"
							placeholder="留空或填写 .env 中的环境变量名"
							value={discord.webhook_url}
							onChange={(value) => updateDiscord({ webhook_url: value })}
						/>
					)}
				</div>

				<Separator />

				{/* Telegram */}
				<div className="flex flex-col gap-3">
					<div className="flex items-center justify-between">
						<Label className="text-xs font-semibold">Telegram</Label>
						<Switch
							checked={telegram.enabled}
							onCheckedChange={(v) => updateTelegram({ enabled: v })}
						/>
					</div>
					{telegram.enabled && (
						<div className="flex flex-col gap-3">
							<SecretField
								id="telegramToken"
								label="Bot Token"
								placeholder="留空或填写 .env 中的环境变量名"
								value={telegram.bot_token}
								onChange={(value) => updateTelegram({ bot_token: value })}
							/>
							<SecretField
								id="telegramChat"
								label="Chat ID"
								placeholder="@channel 或 chat_id"
								value={telegram.chat_id}
								onChange={(value) => updateTelegram({ chat_id: value })}
							/>
						</div>
					)}
				</div>
			</CardContent>
		</Card>
	);
}
