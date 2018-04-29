/*
Package lpo provides Go language suite of tools for Linear Programming (LP) and
Mixed-Integer Linear Programming (MILP). It is intended for two sets of users:
(i) researchers working on LP/MILP algorithms, and (ii) users wanting easy Go
access to the well-known Cplex solver.

Some of the main functions include:
	- ability to read model files in MPS format, or create models directly
	- model presolving
	- evaluating constraints and points
	- solving models via submissions to the Cplex solver

Package lpo indirectly makes use of the callable C functions available in the Cplex
solver by using the auxiliary gpx package.

This package implements some of the presolving techniques to reduce the size 
of a model that is passed to the solver. The techniques, described by Andersen,
are available at https://www.researchgate.net/publication/220589130_Presolving_in_linear_programming. 

The reduction algorithms that are supported by lpo at this time include:

	- non-binding constraints
	- empty rows              (constraint has no variables)
	- row singletons          (constraints that have a single variable)
	- fixed variables         (upper bound equals the lower bound)
	- free column singletons  (unbounded variable present only in the objective function)

The user has control over which of these reduction algorithms get invoked
by setting the appropriate boolean flags and specifying the number of iterations
to be performed. The configurable parameters are:

	type PsCtrl struct {
		FileInputMps      string  // Full path for MPS file containing input
		FileOutputCplex   string  // Path path for Cplex output file, or "" for default
		MaxIter           int     // Maximum iterations for lpo
		DelRowNonbinding  bool    // Controls if non-binding rows are removed
		DelRowSingleton   bool    // Controls if row singletons are removed
		DelColSingleton   bool    // Controls if column singletons are removed
		DelFixedVars      bool    // Controls if fixed variables are removed
		RunCplex          bool    // Controls if problem is to be solved by Cplex 		
	}

Additional reductions will be included in future enhancements.

The user may specify the name of the MPS data file which defines the model if
the model is to be read from file rather than from the internal data structures.
The user may also specify the name of the Cplex data file to which the solution will
be written in xml format so that it may be processed at a later time.

The user has the ability to NOT have Cplex solve the problem if the intent is to 
only perform the reductions provided by this package, but have a different solver
solve the problem. In this case, the "RunCplex" flag is set to false, and other
functions are used to, for example, write the reduced model to a file so it may be
processed by a different solver. 

The expected usage of this package if using the SolveProb function is to set all
relevant fields in the control data structure and have lpo read the model, reduce
the size, solve it via Cplex, and return the results for additional processing.
For example, the code could include the following:

	var ctrl   lpo.PsCtrl
	var result lpo.PsSoln
	...
	// Initialize the control structure.
	ctrl.FileInputMps     = "C:/Data/LP/myModel.txt"
	ctrl.DelRowNonBinding = true
	ctrl.DelRowSingleton  = false
	...

	// Solve the problem. Return if an error condition is detected.	
	if err := lpo.SolveProb(ctrl, &result); err != nil {
		fmt.Printf("lpo returned the following error: %s\n", err)
		return
	}
	...
	
The PsSoln structure merges the results provided by Cplex with the results
calculated outside of Cplex on the constraints and/or variables that were removed
during the presolve operations. Some parameters (e.g. Slack, Dual, ReducedCost)
can only be calculated by Cplex. If the constraint and/or variable
were removed during the presolve operations, Cplex will not be able to calculate
these values, and the post-solve algorithms do not, at this time, have the ability to
calaculate them either. In such cases, the status of the parameter is set to "NA",
and the parameter value is set to 0.

Future enhancements are expected to increase the number of reductions beyond what
the current version supports.

Users who do not wish to reduce and solve the problem as described above have the
option of calling individual functions to only perform a specific task. Those
functions are listed and described in the following sections.

*/
package lpo
