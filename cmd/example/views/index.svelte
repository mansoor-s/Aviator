<script lang="ts">

    import {  Modal, Breadcrumb, BreadcrumbItem, Button, Col, Row } from "sveltestrap/src";
    let isOpen = false;
    const toggle = () => (isOpen = !isOpen);


    import Donkey from "./donkey.svelte";

    export let Myprop;
    console.log("Myprop value: " + Myprop);

    let count = 0;

    export function handleClick () {
        count = count + 1;
    }

</script>

<h2>{@debug $$props}</h2>

<Button color="primary" on:click={toggle}>Hello World!</Button>
<Modal body {isOpen} {toggle} header="Open Modal!">
    <p>I hope you like dialogs</p>
</Modal>


<Button on:click={handleClick}  color="primary" outline>Click Me! {count}</Button>


<Breadcrumb>
    <BreadcrumbItem active>Home</BreadcrumbItem>
</Breadcrumb>

<Breadcrumb>
    <BreadcrumbItem>
        <a href="#home">Home</a>
    </BreadcrumbItem>
    <BreadcrumbItem active>Library</BreadcrumbItem>
</Breadcrumb>

<Breadcrumb>
    <BreadcrumbItem>
        <a href="#home">Home</a>
    </BreadcrumbItem>
    <BreadcrumbItem>
        <a href="#library">Library</a>
    </BreadcrumbItem>
    <BreadcrumbItem active>Foo</BreadcrumbItem>
    <BreadcrumbItem>
        <a href="/a">Ducks</a>
    </BreadcrumbItem>
</Breadcrumb>

<Row>
    <Col>
        <Button color="primary" outline>Hello Bootstrap!</Button>
    </Col>
</Row>


<main>
    <h1><Donkey /></h1>
</main>

<style>
    main {

        text-align: center;
        padding: 1em;
        max-width: 240px;
        margin: 0 auto;
    }
    h1 {
        color: green !important;
        text-transform: uppercase;
        font-size: 4em;
        font-weight: 100;
    }
    @media (min-width: 640px) {
        main {
            max-width: none;
        }
    }
</style>
