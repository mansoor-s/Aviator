<script lang="ts">

    import {  Breadcrumb, BreadcrumbItem, Button, Col, Row } from "sveltestrap/src";


    import Donkey from "./donkey.svelte";

    export let Myprop;

    let count = 0;

    export function handleClick () {
        alert("hello!");
        // calling this function will trigger an
        // update if the markup references `count`
        count = count + 1;
    }

    console.log('Look for this text in your go STDOUT');
</script>

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
    <Donkey />
</main>

<style>
    main {

        text-align: center;
        padding: 1em;
        max-width: 240px;
        margin: 0 auto;
    }
    h1 {
        color: red !important;
        color: #ff3e00;
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
