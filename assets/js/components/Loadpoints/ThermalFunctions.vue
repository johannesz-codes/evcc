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
							:disabled="boostDisabled(fn)"
							@change="setBoost(fn, $event)"
						/>
					</div>
				</div>
			</div>
			<div
				v-if="
					fn.kind === 'spaceHeating' &&
					fn.range &&
					fn.normalTarget !== undefined &&
					fn.boostTarget !== undefined
				"
				class="row"
			>
				<div class="col-sm-4 col-form-label pt-0 pt-sm-2">
					{{ $t("main.thermal.temperatures") }}
				</div>
				<div class="col-sm-8 pe-0">
					<ThermalRange
						:ref="controlId(fn, 'targets')"
						:id="controlId(fn, 'targets')"
						:normal-target="fn.normalTarget"
						:boost-target="fn.boostTarget"
						v-bind="fn.range"
						:disabled="targetsDisabled(fn)"
						:loading="isBusy(fn)"
						@change="setTargets(fn, $event)"
					/>
				</div>
			</div>
			<div v-else-if="fn.kind === 'domesticHotWater'" class="row">
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
			<p v-if="fn.pendingTargetUpdate" class="small text-warning mt-2 mb-0">
				{{ $t("main.thermal.pendingTargetUpdate") }}
			</p>
			<p v-if="invalidTargets(fn)" class="small text-muted mt-2 mb-0">
				{{ $t("main.thermal.invalidTargets") }}
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
import ThermalRange from "./ThermalRange.vue";

export default defineComponent({
	name: "ThermalFunctions",
	mixins: [formatter],
	components: { ThermalRange },
	props: {
		id: { type: String, required: true },
		functions: { type: Array as PropType<ThermalFunction[]>, required: true },
	},
	data() {
		return { busy: [] as string[] };
	},
	methods: {
		controlId(fn: ThermalFunction, control: string) {
			return `thermal-${this.id}-${fn.kind}-${control}`;
		},
		isBusy(fn: ThermalFunction) {
			return this.busy.includes(fn.kind);
		},
		invalidTargets(fn: ThermalFunction) {
			return Boolean(
				fn.range &&
				fn.normalTarget !== undefined &&
				fn.boostTarget !== undefined &&
				(fn.boostTarget <= fn.normalTarget ||
					fn.boostTarget > fn.range.max ||
					fn.boostTarget - fn.normalTarget > 5 + 1e-6)
			);
		},
		boostDisabled(fn: ThermalFunction) {
			return (
				this.isBusy(fn) ||
				(!fn.requested &&
					(!fn.available ||
						fn.pendingRestore ||
						fn.pendingTargetUpdate ||
						this.invalidTargets(fn)))
			);
		},
		configDisabled(fn: ThermalFunction) {
			return this.isBusy(fn) || fn.requested || fn.pendingRestore;
		},
		targetsDisabled(fn: ThermalFunction) {
			return (
				this.isBusy(fn) ||
				!fn.available ||
				fn.ownershipLost ||
				fn.pendingRestore ||
				fn.pendingTargetUpdate
			);
		},
		async setTargets(fn: ThermalFunction, targets: { normal: number; boost: number }) {
			this.busy.push(fn.kind);
			try {
				await api.post(`loadpoints/${this.id}/thermal/${fn.kind}/targets`, targets);
			} catch {
				const controls = this.$refs[this.controlId(fn, "targets")] as
					| InstanceType<typeof ThermalRange>[]
					| undefined;
				controls?.forEach((control) => control.reset());
			} finally {
				this.busy = this.busy.filter((kind) => kind !== fn.kind);
			}
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
