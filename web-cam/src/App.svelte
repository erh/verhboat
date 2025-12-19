<script lang="ts">
 import Start from "./start.svelte";
 import { onMount } from "svelte";

 onMount(() => {
   const interval = setInterval(() => {
     location.reload();
   }, 60 * 60 * 1000); // every hour

   return () => clearInterval(interval);
 });
</script>

<main>
  <Start />
</main>
