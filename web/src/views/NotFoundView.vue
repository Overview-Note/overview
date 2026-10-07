<script setup lang="ts">
import { t } from "../i18n";
</script>

<template>
  <div class="notfound">
    <div class="notfound-card">
      <div class="notfound-code">404</div>
      <h1>{{ t("notFound.title") }}</h1>
      <p>{{ t("notFound.desc") }}</p>
      <RouterLink class="primary notfound-home" :to="{ name: 'home' }">
        {{ t("notFound.home") }}
      </RouterLink>
    </div>
  </div>
</template>
