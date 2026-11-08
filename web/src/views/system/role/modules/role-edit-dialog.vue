<template>
  <ElDialog
    v-model="visible"
    :title="dialogType === 'add' ? '新增角色' : '编辑角色'"
    width="520px"
    align-center
    destroy-on-close
  >
    <ElForm ref="formRef" :model="form" :rules="rules" label-width="100px">
      <ElFormItem label="角色名称" prop="name">
        <ElInput v-model.trim="form.name" placeholder="请输入角色名称" maxlength="64" />
      </ElFormItem>
      <ElFormItem label="角色编码" prop="code">
        <ElInput
          v-model.trim="form.code"
          placeholder="如 ops / auditor"
          :disabled="dialogType === 'edit'"
          maxlength="64"
        />
      </ElFormItem>
      <ElFormItem label="数据范围" prop="dataScope">
        <ElSelect v-model="form.dataScope" placeholder="请选择数据范围">
          <ElOption :value="1" label="全部数据" />
          <ElOption :value="2" label="本部门数据" />
          <ElOption :value="3" label="本部门及以下" />
          <ElOption :value="4" label="仅本人数据" />
          <ElOption :value="5" label="自定义部门" />
        </ElSelect>
      </ElFormItem>
      <ElFormItem label="可管理小区">
        <div class="flex w-full flex-wrap items-center gap-2">
          <ElRadioGroup v-model="form.communityScope" size="small">
            <ElRadioButton :value="1">本物业公司全部小区</ElRadioButton>
            <ElRadioButton :value="2">指定小区</ElRadioButton>
          </ElRadioGroup>
          <ElSelect
            v-if="form.communityScope === 2"
            v-model="form.communityIds"
            multiple
            filterable
            collapse-tags
            placeholder="选择可管理的小区"
            class="!w-64"
          >
            <ElOption v-for="c in communityOptions" :key="c.id" :label="c.name" :value="c.id" />
          </ElSelect>
        </div>
      </ElFormItem>

      <ElFormItem label="排序">
        <ElInputNumber v-model="form.sort" :min="0" :max="9999" controls-position="right" />
      </ElFormItem>
      <ElFormItem label="状态">
        <ElRadioGroup v-model="form.status">
          <ElRadio :value="1">正常</ElRadio>
          <ElRadio :value="2">停用</ElRadio>
        </ElRadioGroup>
      </ElFormItem>
      <ElFormItem label="描述">
        <ElInput
          v-model="form.remark"
          type="textarea"
          :rows="3"
          maxlength="200"
          show-word-limit
          placeholder="请输入角色描述"
        />
      </ElFormItem>
    </ElForm>

    <template #footer>
      <ElButton @click="visible = false">取消</ElButton>
      <ElButton type="primary" :loading="submitting" @click="handleSubmit">提交</ElButton>
    </template>
  </ElDialog>
</template>

<script setup lang="ts">
  import type { FormInstance, FormRules } from 'element-plus'
  import { fetchCreateRole, fetchUpdateRole, fetchRoleCommunityOptions, fetchGetRoleGranted } from '@/api/system-manage'

  type RoleListItem = Api.SystemManage.RoleListItem

  interface Props {
    modelValue: boolean
    dialogType: 'add' | 'edit'
    roleData?: RoleListItem
  }

  const props = withDefaults(defineProps<Props>(), {
    modelValue: false,
    dialogType: 'add',
    roleData: undefined
  })
  const emit = defineEmits<{
    (e: 'update:modelValue', value: boolean): void
    (e: 'success'): void
  }>()

  const visible = computed({
    get: () => props.modelValue,
    set: (value) => emit('update:modelValue', value)
  })

  const formRef = ref<FormInstance>()
  const submitting = ref(false)

  const form = reactive({
    name: '',
    code: '',
    dataScope: 4,
    communityScope: 1,
    communityIds: [] as string[],
    sort: 0,
    status: 1,
    remark: ''
  })
  const communityOptions = ref<Array<{ id: string; name: string }>>([])

  const rules = reactive<FormRules>({
    name: [
      { required: true, message: '请输入角色名称', trigger: 'blur' },
      { min: 2, max: 64, message: '长度在 2 到 64 个字符', trigger: 'blur' }
    ],
    code: [
      { required: true, message: '请输入角色编码', trigger: 'blur' },
      { pattern: /^[a-zA-Z][\w-]*$/, message: '以字母开头，可包含字母数字下划线和横线', trigger: 'blur' }
    ],
    dataScope: [{ required: true, message: '请选择数据范围', trigger: 'change' }]
  })

  watch(
    () => props.modelValue,
    (val) => {
      if (!val) return
      formRef.value?.clearValidate()
      fetchRoleCommunityOptions()
        .then((list) => (communityOptions.value = (list || []) as any))
        .catch(() => undefined)
      if (props.dialogType === 'edit' && props.roleData) {
        fetchGetRoleGranted(props.roleData.roleId)
          .then((g: any) => {
            form.communityScope = g?.communityScope ?? 1
            form.communityIds = (g?.communityIds || []).map(String)
          })
          .catch(() => undefined)
        form.name = props.roleData.roleName
        form.code = props.roleData.roleCode
        form.dataScope = props.roleData.dataScope ?? 4
        form.sort = props.roleData.sort ?? 0
        form.status = props.roleData.enabled ? 1 : 2
        form.remark = props.roleData.description || ''
      } else {
        Object.assign(form, {
          name: '',
          code: '',
          dataScope: 4,
          communityScope: 1,
          communityIds: [],
          sort: 0,
          status: 1,
          remark: ''
        })
      }
    }
  )

  const handleSubmit = async () => {
    if (!formRef.value) return
    const valid = await formRef.value.validate().catch(() => false)
    if (!valid) return

    submitting.value = true
    try {
      const payload = {
        name: form.name,
        code: form.code,
        dataScope: form.dataScope,
        communityScope: form.communityScope,
        communityIds: form.communityScope === 2 ? form.communityIds : [],
        sort: form.sort,
        status: form.status,
        remark: form.remark
      }
      if (props.dialogType === 'add') {
        await fetchCreateRole(payload)
      } else {
        await fetchUpdateRole(props.roleData?.roleId as string, payload)
      }
      emit('success')
      visible.value = false
    } finally {
      submitting.value = false
    }
  }
</script>
