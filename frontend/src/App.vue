<template>
  <div class="app">
    <header class="header">
      <h1>🎬 实时直播</h1>
    </header>
    <main class="main">
      <Player />
    </main>
  </div>
</template>

<script setup>
import Player from './components/Player.vue'
</script>

<style>
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  background: #0a0a0a;
  color: #fff;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
}

.app {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

.header {
  padding: 16px 24px;
  background: #111;
  border-bottom: 1px solid #222;
}

.header h1 {
  font-size: 1.25rem;
  font-weight: 600;
  color: #fff;
}

.main {
  flex: 1;
  display: flex;
  justify-content: center;
  align-items: flex-start;
  padding: 24px;
}
</style>
