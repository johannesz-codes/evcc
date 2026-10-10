<template>
	<div class="mb-3">
		<h6>{{ $t("main.thermal.title") }}</h6>
		<p class="small text-muted">{{ $t("main.thermal.sharedPlant") }}</p>
		<section
			v-for="fn in functions"
			:key="fn.kind"
			:aria-label="$t(`main.thermal.${fn.kind}`)"
			class="mb-3"
		>
			<div class="row mb-2">
				<div class="col-sm-4">{{ $t(`main.thermal.${fn.kind}`) }}</div>
				<div class="col-sm-8 d-flex align-items-center gap-2" aria-live="polite">
					<strong v-if="fn.target !== undefined">{{ fmtTemperature(fn.target) }}</strong>
					<strong v-else-if="fn.mode">{{ $t(`main.thermal.modes.${fn.mode}`) }}</strong>
					<span v-else>{{ $t("main.thermal.unavailable") }}</span>
					<small v-if="fn.active" class="text-evcc">{{
						$t("main.thermal.active")
					}}</small>
				</div>
			</div>
			<div class="row align-items-center mb-2">
				<label class="col-sm-4 col-form-label pt-0 pt-sm-2" :for="controlId(fn, 'boost')">{{
					$t("main.thermal.boost")
				}}</label>
				<div class="col-sm-8">
					<div class="form-check form-switch mb-0">
						<input
							:id="controlId(fn, 'boost')"
							type="checkbox"
							role="switch"
							class="form-check-input"
							:checked="fn.requested"
							:disabled="isBusy(fn) || (!fn.available && !fn.requested)"
							@change="setBoost(fn, $event)"
						/>
					</div>
				</div>
			</div>
			<div v-if="fn.kind === 'spaceHeating'" class="row">
				<label :for="controlId(fn, 'delta')" class="col-sm-4 col-form-label pt-0 pt-sm-2">{{
					$t("main.thermal.delta")
				}}</label>
				<div class="col-sm-8 col-lg-4 pe-0">
					<select
						:id="controlId(fn, 'delta')"
						class="form-select form-select-sm"
						:value="fn.boost.delta"
						:disabled="configDisabled(fn)"
						@change="setConfig(fn, $event)"
					>
						<option v-for="delta in deltas" :key="delta" :value="delta">
							+{{ fmtNumber(delta, 1) }} K
						</option>
					</select>
				</div>
			</div>
			<div v-else class="row">
				<label :for="controlId(fn, 'mode')" class="col-sm-4 col-form-label pt-0 pt-sm-2">{{
					$t("main.thermal.boostMode")
				}}</label>
				<div class="col-sm-8 col-lg-4 pe-0">
					<select
						:id="controlId(fn, 'mode')"
						class="form-select form-select-sm"
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
import formatter from "@/mixins/formatter";

export default defineComponent({
	name: "ThermalFunctions",
	mixins: [formatter],
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
