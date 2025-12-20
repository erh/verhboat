<script lang="ts">
 import Start from "./start.svelte";
 import { onMount } from "svelte";

 onMount(() => {
   const interval = setInterval(() => {
     location.reload();
   }, 7 * 24 * 60 * 60 * 1000); // every week

   return () => clearInterval(interval);
 });
</script>

<main>
  <Start />
</main>
