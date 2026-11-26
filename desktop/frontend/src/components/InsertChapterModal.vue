<script setup lang="ts">
import { ref } from 'vue'

const props = defineProps<{
  open: boolean
  maxLines: number
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'submit', lineNo: number, title: string): void
}>()

const lineNo = ref(1)
const title = ref('')

function handleSubmit() {
  emit('submit', lineNo.value, title.value)
  title.value = ''
}
</script>

<template>
  <div v-if="props.open" class="modal-mask" @click.self="emit('close')">
    <div class="modal-box">
      <h3>按行号手动插入章节</h3>
      <div class="field">
        <span class="field-label">
          <span>起始行号 (1 ~ {{ props.maxLines }})</span>
        </span>
        <input
          v-model.number="lineNo"
          class="input"
          type="number"
          min="1"
          :max="props.maxLines"
        />
      </div>
      <div class="field">
        <span class="field-label">
          <span>章节标题</span>
          <span class="field-hint">留空则取该行文本</span>
        </span>
        <input v-model="title" class="input" type="text" placeholder="例如：第12章 补遗" />
      </div>
      <div class="modal-actions">
        <button class="btn" type="button" @click="emit('close')">取消</button>
        <button class="btn btn-primary" type="button" @click="handleSubmit">确定插入</button>
      </div>
    </div>
  </div>
</template>
