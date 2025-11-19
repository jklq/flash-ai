[back to main page](../README.md)

# Assembly Language Exploration with Compiler Explorer

## Objective

In this group session, you will explore how **C** code is translated into assembly language across different architectures and compilers. By understanding the assembly output, you will gain insights into how different optimizations, data types, and architectures affect code execution.

## Get started with the Compiler
1. Open the compiler here: https://godbolt.org/.
2. Change the language from **C++** to **C**. There's a field for this at the top of the code input section on the left.
3. Find the compiler output options (the cog that says **Output...**). Untick **Intel asm syntax** as this is what we will be interpreting for the rest of the session. The output now matches the lecture examples.
4. You can add multiple compilers to easily make comparisons by simply clicking on **+ Add new...** on the same bar where you found **Output...** and selecting **Compiler**. <br>

You will come across a lot of new abbreviations for instructions and pointers/registers. You do not really need to understand what each of them, is but being able to distinguish between intructions and pointer/registers would be of help.

## Compiler Tasks

1. In the Compiler Explorer write a **C** function that performs basic addition:

```c
int add(int a, int b){
    return a + b;
}
```

2. Let this function compile using **x86-64**, **clang** and **gcc** (latest versions) for different optimization levels (compiler options or flags) **-O0**, **-O1**, **-O2**, and analyze the assembly output. Observe how the assembly code changes with optimization.  We'll get back to %rbp and %rsp later, don't worry too much about those, for now see them as "locations". 4(%rsp) is 4 bytes away from 8(%rsp) etc.

3. Switch to **arm gcc** and repeat the compilation and comparision for different levels of optimization. Observe how the assembly code for **x86-64** differs to that of **ARM**.

4. Input this function in addition to the add function to the code area:

```c
int add_constants(){
    int x = 20;
    int y = 22;
    return add(x,y);
}
```

5. Observe how its assembly code changes for different optimization levels: **-O0**, **-O1**, **-O2**.  Now also analyze it with the flag **-fno-inline**. Try and understand what the compiler achieves with the different flags used here.

6. Explore how the assembly code changes when the addition is replaced with different operations such as subtraction, multiplication and division.

7. Try declaring more variables in the functions **add** and **add_constants** which do not get used by the return and see what happens in the assembly code with different levels of optimization. Try operating with those constants within the functions without affecting the return and see how the assembly code changes with different levels of optimization.

8. Find out how the assembly code changes the instructions it chooses and the way it operates when working with other data types such as **double**.

9. Try a messier function such as the one below and try and understand how assembly code (and indirectly binary code) handles **if-else** statements:

```c
int special_add(int a, int b){
    if(a>b){
        return a + 2*b;
    }
    else{
        return 2*a +b;
    }
    return 2*(a+b);
}
```

10. **(optional)** Have you heard about vectorization before? Declare the function provided below and set the flags to **-O1**, **-O2** and observe how the asembly code differs to having set the flags to to **-O2** **-mavx**. Now change the size of the loop, try 4, 8, 16. How many add-operations are run in the assembly?

```c
void add_arrays(float *a, float b) {
    for (int i = 0; i < 8; i++) {
        a[i] += b;
    }
}
```


## Reflection Questions

1.	How does the assembly code differ between the optimization levels?

2.	How do the instructions differ between **x86-64** and **ARM** for the same **C** code?

3.	What is the purpose of assembly code that is not optimized?

4.	How does the compiler handle fixed variables and variable variables when running with higher optimization flags?

5.	How are conditional statements translated into assembly?

6.	**(optional)** You must have noticed the repeated use of **rbp**, **rsp**, **edi**, **esi** and **eax**. What are they called? Find out their purpose and in which order they are used for the different operations. Why are they named with 3 letters? Interoperability?

7.	**(optional)** In the assembly code, you may have noticed negative integers in front of **rbp**, what do they refer to?
