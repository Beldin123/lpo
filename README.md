# Exported Functions

Package LPO provides a Go language suite of tools for Linear Programming (LP) and Mixed-Integer Linear Programming (MILP). It is intended for two sets of users: (i) researchers working on LP/MILP algorithms, and (ii) users wanting easy Go access to the well-known Cplex solver. Some of the main functions include:

*	Ability to read model files in MPS format, or to create models directly,
*	Model presolving,
*	Evaluating constraints and points,
*	Solving models via submission to the Cplex solver.

LPO indirectly makes use of the callable C functions available in the Cplex solver by using the independent GPX package.

# Dependencies

The lpo package is dependent on the gpx package and the errors package, both of which are available in github and must be
downloaded separately. The import statments are:

*	github.com/pkg/errors
*	github.com/go-opt/gpx (if using the callable C functions provided by Cplex)
*	OSSolverService (if using the Coin-OR solver)

The gpx package is itself dependent on the installation and configuration of Cplex. Please refer to that package for details.

# Executable

The lporun subdirectory contains the executable intended as a tutorial demonstrating how lpo and gpx functions are used as well
as exercisers which permit users to independently call and test each function exported by the two packages.

# Installation and Configuration

To install the package on a Windows platform, go to the cmd.exe window and enter the command:

  go get -u github.com/go-opt/lpo

The lpo package may be used independently (without any solvers) if you only wish to read and manipulate, but not solve, a model. If you also wish to solve the model, one (or both) of the solvers must also be installed.

When the required software is installed and configured, build the package and run the executable lporun to 
see the examples of how this package can be used.

## Configuring lpo for Coin-OR

If lpo is used with Coin-OR, you must install the OSSolverService, which is part of the Coin-OR suite of executables. 
Once installed, the package global variable in file ifcoin.go must be modified with the correct location of this 
executable. That is, you must change the following line so that it contains the correct path to the executable:

  var coinOrExe string = "C:/coin_dir/OSSolverService"

If you do not wish to use the Coin-OR solver, you need not make any changes in the ifcoin.go or any other file. 
All functions not associated with Coin-OR will work correctly, and those functions associated with Coin-OR will return 
an error if called.

## Configuring lpo for gpx 

If lpo is used with the gpx package, go to the cmd.exe window and install gpx using the following command:

  go get -u github.com/go-opt/gpx

You need to install and configure Cplex, and you also need to configure gpx. Please refer to the instructions
provided in the gpx package for details.

## Configuring lpo WITHOUT gpx

If gpx is not installed, you need to modify some lpo files so that the other functions not using gpx may be compiled and
executed.

File ifgpx.go in the lpo directory and file utilsgpx.go in the lporun directory must be excluded from being built 
by uncommenting the first line of that file so that it reads:

  // +build exclude

In the lporun.go file in the lporun directory, you must comment out the reference to the wpCplexSolveProb function
so that the block of code reads as follows:

  // Comment out the following line if gpx is not installed and
  // the utilsgpx.go file is excluded from being built.
  // err = wpCplexSolveProb()

Once you have made these changes, you can compile and use lpo without gpx.

# Development and Testing

The lpo and gpx packages were developed and tested using the following software

* Windows 7 Service Pack 1
* golang, version 1.8.3
* LiteIDE X32.2, version 5.6.2 (32-bit)
* MinGW-w64 64/32-bit C compiler, version 5.1.0
* Cplex, version 12.7.1
* Coin-OR, version 1.7.4-win32-msvc11

Testing was performed using LP problems provided by netlib (http://www.netlib.org/lp/data/index.html) and some (not all)
MILP problems provided by miplib (http://miplib.zib.de/).
