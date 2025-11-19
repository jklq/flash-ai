[back to main page](../README.md)

# Programming and Running Programs in the Virtual Machine

## Objective

In this group session, you will write a few simple programs in **C** and run them in the terminal in your virtual machine.
By following the steps below, you will solidify your knowledge of the basics of the C language that we covered on the lectures: data types, loops and conditional operators, functions, I/O. You will also learn a little bit about how larger C projects are organized, with makefiles and libraries. Finally, you will become more confident with using the VM and the terminal, which will be useful for running the code examples from the book, going through the consequent homeworks and solving the mandatory assignments.

## Starting with the first program
1. Open the terminal, create a folder for the programs in this exercise, and move to it. For example,
```
mkdir ~/week02/
cd ~/week02/
```
2. Edit the file `mult.c` with your favourite text editor, for example,
```
gedit mult.c &
```

Note the `&` symbol that keeps the original terminal prompt open.

3. Start by entering a blank C program in the file:
```
#include <stdio.h>
#include <stdlib.h>

int main(int argc, char *argv[]) {
    //TODO your code here

    return 0;
}

```
4. While the program does not do anything useful yet, it is syntactically correct, so it should compile. Verify this by running
```
gcc -o mult mult.c
```
Now, run the compiled program by
```
./mult
```
5. You are now ready to actually write the program. The program should read two integers from the standard input, and print their product to the standard output.
6. When you are done, compile the program again
```
gcc -o mult mult.c -Wall
```
Note the `-Wall` flag that will show you all possible warnings that the compiler can come up with. It is always recommended to enable warnings, since seeing them makes it easier to find mistakes.

7. If the program compiles, make sure that it works correctly by running it. The output should look like the following:
```
./mult
2 3
6
```
Test the program for at least a few other cases: positive and negative factors, zero factors, etc.

**Bonus question:** Can you find an example that does not work correctly?

## Strings and files

1. Set up a file for another program, `atof.c`. The program should read a single string containing lowercase letters, replace all `a` letters with `f` letters, and output the resulting string.
2. Compile the program
```
gcc -o atof atof.c -Wall
```
Note that if you use string functions, for example `strlen`, you will likely need to include the `string.h` header from the standard library. With the `-Wall` flag, the compiler itself will remind you that.

3. Running the program should look as follows.
```
./atof
atof
ftof
```
3. Observe that entering a string each time you want to test the program is tiring. Instead, save the string to a file: Create a file `1.in` containing a single input to your program. You can do so either normally with a text editor, i.e., `gedit 1.in`, or by dumping the string directly from the command line:
```
echo "atof" > 1.in
```
You can then see the contents of your file from the command line with
```
cat 1.in
```
4. Now, test your program without manually entering the string. Run
```
./atof < 1.in
```
The output should be exactly as in the manual test, of course.
You can also save the output to a file in case you would like to verify it automatically, or just at a later time:
```
./atof < 1.in > 1.out
```

5. Finally, let us modify our program to take its input from the arguments, instead of the standard input. Copy your current program to a new file:
```
cp atof.c atof_args.c
```
6. Recall that the command-line arguments are passed as arguments to the `main` function. Modify `atof_args.c` so that it takes the input string as the command-line argument. That is, after compilation, the program should run as follows:
```
./atof_args atof
ftof
```
Almost every command-line utility has to look up its arguments in the same way that we just did.

## Functions, headers, and makefiles

Let us reuse the code written in `atof.c` for a more general purpose. Clearly, in the same way one can replace all occurences of any given character by another character. We will extract this logic to a function that will be defined in its own file.

1. Create a file `replace.c` that contains a single function:
```
void replace(char* s, char from, char to) {
    //TODO your code here
}
```
Instead of the commentary, implement the function that iterates over all characters in the string `s`, and replaces any `from` character with the `to` character.

2. In order for other programs to use the `replace` function, we will need to define it in a separate header file. Create a file `replace.h` that contains only the following line:
```
void replace(char* s, char from, char to);
```
3. Now modify your `atof.c` program to make use of the `replace` function. You will need to include the `replace.h` header after the other includes, and then you can simply call `replace`. That is, your program will likely be of the following form:
```
//TODO other includes
#include "replace.h"

int main(int argc, char *argv[]) {
    //TODO defining and reading s
    replace(s, 'a', 'f');
    //TODO printing s

    return 0;
}
```

4. Try to compile `atof.c` like before. Note that it does not work since the code from `replace.c` is missing.

5. To fix this, one can compile both codes at the same time:
```
gcc -o atof atof.c replace.c -Wall
```
Test that `./atof` works just as well as before. Now one can easily implement `atob.c`, `atoc.c`, ..., or even `ftoa.c` with the help of our new `replace` function! 

6. To streamline the build process, let us create a _makefile_. A makefile defines a collection of _targets_, i.e., individual actions. As a simple application, we may add our compilation command to avoid retyping it every time.

7. Open a new file named `Makefile` and add the following lines to it:
```
compile:
	gcc -o atof atof.c replace.c -Wall

run:
	./atof
```
The indentation **must** be with the tab character and not spaces!

8. Now, run
```
make compile
```
and
```
make run
```
Note that it performs exactly the specified actions.

9. Enhance the makefile to look as follows:
```
all: compile run

compile:
	gcc -o atof atof.c replace.c -Wall

run:
	./atof

clean:
	rm atof
```
Now we can also use `make all` to compile the project and run it afterwards, and `make clean` to remove the compiled binary file, reverting the build process. Try these commands.

10. Finally, we will learn to use _prerequisites_, a powerful feature of `make` that allows it to only run the target if necessary, for example, to avoid recompiling the binary when the source file wasn't changed. To enable this for our `compile` target, write instead
```
atof: atof.c replace.c
	gcc -o atof atof.c replace.c -Wall

compile: atof
```
This will let `make` know that `atof` should be recompiled only when either `atof.c` or `replace.c` changes. The commands `make atof` and `make compile` now result in the same actions. We may also add the `compile` prerequisite to the `run` target, so that the executable is only run after the successful (re-)compilation.
```
run: compile
	./atof
```
Try out the new commands. Observe also that when compilation failes during `make run`, `make` rightfully skips running the executable.



## Bonus objective: create a static library

The steps described above achieve encapsulation from the code viewpoint—the `replace` function is separated away in its own file, and its header file `replace.h` provides the signature of the function to other files. However, in terms of the compilation it is as if the whole project is still inseparable: whenever we change anything either in `atof.c` or in `replace.c`, the whole binary executable will be recompiled from scratch.

While this is not a big issue for our little program, in real projects that may span many thousands or millions lines of code, recompiling everything with every small change would be extremely inefficient. Moreover, since the C standard library is in itself just a lot of C code, compiling even a small program in this fashion would take a long time—as all pieces of the standard library would have to be compiled as well!

That is why it is actually possible to compile a library separately, and then _link_ the compiled binary into the main project whenever it is compiled, without recompiling the library itself. 

Learn how to do that with our `replace` function: compile it into a binary library so that `atof.c` or any other program that may use this function will not need the `replace.c` file to be compiled again. In fact, once the library is compiled, the `replace.c` source file will no longer be necessary at all—try moving it outside of the folder. The header file `replace.h` will still be necessary, however. Keyword: "static library in C".
