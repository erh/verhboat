<script lang="ts">
 import Start from "./start.svelte";
 import { onMount } from "svelte";

 onMount(() => {
   const interval = setInterval(() => {
     location.reload();
   }, 10 * 60 * 1000); // 10 minutes

   return () => clearInterval(interval);
 });
</script>

<main>
  <Start />
</main>
