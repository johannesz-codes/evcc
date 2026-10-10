<template>
	<div class="mb-3">
		<p class="small text-muted">{{ $t("main.thermal.sharedPlant") }}</p>
		<section
			v-for="fn in functions"
			:key="fn.kind"
			:aria-label="$t(`main.thermal.${fn.kind}`)"
			class="border-top py-3"
		>
			<div class="d-flex justify-content-between align-items-center gap-2 mb-2">
				<h4 class="h6 mb-0">{{ $t(`main.thermal.${fn.kind}`) }}</h4>
				<span v-if="fn.active" class="badge bg-success">{{
					$t("main.thermal.active")
				}}</span>
			</div>
			<div class="mb-2 small" aria-live="polite">
				{{ $t("main.thermal.currentTarget") }}:
				<span v-if="fn.target !== undefined">{{ fn.target.toFixed(1) }} °C</span>
				<span v-else-if="fn.mode">{{ $t(`main.thermal.modes.${fn.mode}`) }}</span>
				<span v-else>{{ $t("main.thermal.unavailable") }}</span>
			</div>
			<div class="d-flex align-items-center justify-content-between flex-wrap gap-2">
				<div class="form-check form-switch">
					<input
						:id="controlId(fn, 'boost')"
						type="checkbox"
						role="switch"
						class="form-check-input"
						:checked="fn.requested"
						:disabled="isBusy(fn) || (!fn.available && !fn.requested)"
						@change="setBoost(fn, $event)"
					/>
					<label class="form-check-label" :for="controlId(fn, 'boost')">{{
						$t("main.thermal.boost")
					}}</label>
				</div>
				<div v-if="fn.kind === 'spaceHeating'" class="d-flex align-items-center gap-2">
					<label :for="controlId(fn, 'delta')" class="small">{{
						$t("main.thermal.delta")
					}}</label>
					<select
						:id="controlId(fn, 'delta')"
						class="form-select form-select-sm w-auto"
						:value="fn.boost.delta"
						:disabled="configDisabled(fn)"
						@change="setConfig(fn, $event)"
					>
						<option v-for="delta in deltas" :key="delta" :value="delta">
							+{{ delta.toFixed(1) }} K
						</option>
					</select>
				</div>
				<div v-else class="d-flex align-items-center gap-2">
					<label :for="controlId(fn, 'mode')" class="small">{{
						$t("main.thermal.boostMode")
					}}</label>
					<select
						:id="controlId(fn, 'mode')"
						class="form-select form-select-sm w-auto"
						:value="fn.boost.mode"
						:disabled="configDisabled(fn)"
						@change="setConfig(fn, $event)"
					>
						<option v-for="mode in boostModes(fn)" :key="mode" :value="mode">
							{{ $t(`main.thermal.modes.${mode}`) }}
						</option>
					</select>
				</div>
			</div>
			<p v-if="fn.ownershipLost" class="small text-warning mt-2 mb-0">
				{{ $t("main.thermal.ownershipLost") }}
			</p>
			<p v-if="fn.pendingRestore" class="small text-warning mt-2 mb-0">
				{{ $t("main.thermal.pendingRestore") }}
			</p>
			<p v-if="!fn.available" class="small text-muted mt-2 mb-0">
				{{ $t("main.thermal.unavailable") }}
			</p>
		</section>
	</div>
</template>

<script lang="ts">
import { defineComponent, type PropType } from "vue";
import api from "@/api";
import type { ThermalFunction } from "@/types/evcc";

export default defineComponent({
	name: "ThermalFunctions",
	props: {
		id: { type: String, required: true },
		functions: { type: Array as PropType<ThermalFunction[]>, required: true },
	},
	data() {
		return { busy: [] as string[], deltas: Array.from({ length: 50 }, (_, i) => (i + 1) / 10) };
	},
	methods: {
		controlId(fn: ThermalFunction, control: string) {
			return `thermal-${this.id}-${fn.kind}-${control}`;
		},
		isBusy(fn: ThermalFunction) {
			return this.busy.includes(fn.kind);
		},
		configDisabled(fn: ThermalFunction) {
			return this.isBusy(fn) || fn.requested || fn.pendingRestore;
		},
		boostModes(fn: ThermalFunction) {
			return fn.boostModes ?? [];
		},
		async setBoost(fn: ThermalFunction, event: Event) {
			const input = event.target as HTMLInputElement;
			const requested = input.checked;
			this.busy.push(fn.kind);
			try {
				await api.post(`loadpoints/${this.id}/thermal/${fn.kind}/boost/${requested}`);
			} catch {
				input.checked = fn.requested;
			} finally {
				this.busy = this.busy.filter((kind) => kind !== fn.kind);
			}
		},
		async setConfig(fn: ThermalFunction, event: Event) {
			const input = event.target as HTMLSelectElement;
			const config =
				fn.kind === "spaceHeating" ? { delta: Number(input.value) } : { mode: input.value };
			this.busy.push(fn.kind);
			try {
				await api.post(`loadpoints/${this.id}/thermal/${fn.kind}/config`, config);
			} catch {
				input.value = String(fn.boost.delta ?? fn.boost.mode);
			} finally {
				this.busy = this.busy.filter((kind) => kind !== fn.kind);
			}
		},
	},
});
</script>
