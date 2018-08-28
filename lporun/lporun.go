//==============================================================================
// lporun: Executable for running some lpo and gpx functions.
// 01 - Jul.  4, 2018   First version, uploaded to github
// 02   Aug. 28, 2018   Simplified to reduce complexity and remove functionality


// This file contains wrapper functions demonstrating how some lpo functions are 
// used except those which require gpx to be installed.


package main

import (
	"fmt"
	"github.com/go-opt/lpo"
	"github.com/pkg/errors"
	"time"
)

// Default input and output files. If the files are in a different directory
// than the one where the executable is launched, full absolute path must be used.
var inputSmLP   string = "inputSmallLp.txt"    // small LP example MPS file
var inputSmMILP string = "inputSmallMilp.txt"  // small MILP example MPS file      
var inputLg     string = "inputLargeLP.txt"    // large example MPS file
var outSoln     string = "soln_file.txt"       // solution output file
var outPsop     string = "psop_file.txt"       // PSOP output file
var outRedMtx   string = "rmx_file.txt"        // reduced matrix output file

// Other useful package global variables. The statistics and solution data structures
// are global to avoid having to pass them in function calls (as SHOULD be done)
// in this sample program.
var pauseAfter    int  = 50    // number of items to print before pausing
var lpStats   lpo.Statistics   // statistics data structure
var lpCpSoln  lpo.CplexSoln    // Cplex solution obtained from parsing xml file
var psResult  lpo.PsSoln       // solution received from lpo


//==============================================================================

// printOptions displays the options that are available for testing. Package
// global flags control which menus are printed.
// The function accepts no arguments and returns no values.
func printOptions() {

	fmt.Println("\nAvailable Options:\n")

	fmt.Println(" 0 - EXIT program")
	fmt.Println(" 1 - read, print, and reduce model without solving")
	fmt.Println(" 2 - solve small LP problem using Coin-OR CLP solver")
	fmt.Println(" 3 - solve small MILP problem using Coin-OR CBC solver")
	fmt.Println(" 4 - solve large problem using Cplex and gpx")
	fmt.Println(" 5 - display lpo solution")
	
}

//==============================================================================

// wpInitLpo initializes all input, solution, and other (e.g. statistics) data
// structures. As much as possible, it uses the initialization routines from the
// lpo package. The function accepts no arguments and returns no values.
func wpInitLpo() {

	lpo.InitModel()

	// If the model is empty, and we get the statistics, we actually initialize
	// the statistics data structure (since there is nothing to get).
	
	lpo.GetStatistics(&lpStats)

	// Similarly, if we call CplexParseSoln with a bogus file name and ignore the
	// error, we get back the initialized Cplex solution data structure (again
	// because there is nothing to get).
	
	_ = lpo.CplexParseSoln("", &lpCpSoln)
				
	// The only thing left to initialize is the solution data structure.
		
    psResult.ColsDel = 0
	psResult.RowsDel = 0
	psResult.ElemDel = 0
	psResult.ObjVal  = 0.0
	psResult.ConMap  = nil
	psResult.VarMap  = nil
		
}

//==============================================================================

// wpPrintLpoSoln prints the solution contained in the lpo data structures. It
// presents the data in a formatted manner, and gives the user the option to pause
// periodically so output does not scroll off the screen. The function accepts no
// input and returns no values.
func wpPrintLpoSoln() {
	var userString string
	var counter int
	var index   int


	fmt.Printf("\nOBJECTIVE FUNCTION = %f\n\n", psResult.ObjVal)

	// Check if the lists exist, and if they do, print them.
					
	if len(psResult.VarMap)	<= 0 {
		fmt.Printf("WARNING: Solution list of variables is empty.\n")
	} else {
		userString = ""
		fmt.Printf("\nDisplay variable list [Y|N]: ")
		fmt.Scanln(&userString)
		if userString == "y" || userString == "Y" {
			fmt.Printf("Variables are:\n")
			fmt.Printf("%6s  %-10s     %15s %15s %15s\n", "INDEX", "NAME", "VALUE", 
				"REDUCED COST", "SCALE FACTOR")
			
			counter = 0
			index   = 0
			for psVarbName, psVarb := range psResult.VarMap {
				fmt.Printf("%6d  %-10s     %15e %15e %15e\n", index, psVarbName,
					psVarb.Value, psVarb.ReducedCost, psVarb.ScaleFactor)
					
				counter++
				index++
				if counter == pauseAfter {
					counter = 0
					userString = ""
					fmt.Printf("\nPAUSED... <CR> continue, any key to quit: ")
					fmt.Scanln(&userString)
					if userString != "" {
						break 
					}
				} // end if pause required
			} // end for varb range		
		} // end if printing varb list
	} // end else varb list not empty	

	if len(psResult.ConMap) <= 0 {
		fmt.Printf("WARNING: Solution list of constraints is empty.\n")		
	} else {
		userString = ""
		fmt.Printf("\nDisplay constraint list [Y|N]: ")
		fmt.Scanln(&userString)
		if userString == "y" || userString == "Y" {
			fmt.Printf("\nConstraints are:\n")
			fmt.Printf("%6s  %-10s %3s %15s %15s %15s %15s %15s\n", "INDEX", "ROW",
					"EQ", "RHS", "SLACK", "PI", "DUAL", "SCALE FACTOR")
				
			counter = 0
			index   = 0
			for psConName,psCon := range psResult.ConMap {
				fmt.Printf("%6d  %-10s %3s %15e %15e %15e %15e %15e\n",
					index, psConName, psCon.Type,
					psCon.Rhs, psCon.Slack, psCon.Pi, psCon.Dual, psCon.ScaleFactor)
				counter++
				index++
				if counter == pauseAfter {
					counter = 0
					userString = ""
					fmt.Printf("\nPAUSED... <CR> continue, any key to quit: ")
					fmt.Scanln(&userString)
					if userString != "" {
						break 
					}
				} // end if pause required
			} // end for range of cons			
		} // end if printing constraint list
	} // end else constraint list not empty						
	
}

//==============================================================================

// wpPauseOutput is used to pause output at specific points so it does not scroll
// off the screen before the user has a chance to see it.
// The function accepts no arguments. The function returns an error which is
// interpretted by the calling function as a desire to abort the operation in progress.
func wpPauseOutput() error {
	var userString  string
	
	fmt.Printf("Enter 'q' to abort, any other key to continue: ")
	fmt.Scanln(&userString)
	if userString == "q" || userString == "Q" {
		return errors.New("Aborted by user")		
	}

	return nil	
}


//==============================================================================

// wpShowProb illustrates how a problem is read from MPS file, displayed,
// reduced, but not solved. The function accepts no arguments.
// In case of failure, function returns an error.
func wpShowProb() error {
	var psCtrl          lpo.PsCtrl  // control structure for reductions
	var err                  error  // error received from called functions

	fmt.Printf("\nThis example illustrates how to read the model definition from an\n")
	fmt.Printf("MPS file, display the model and statistics about the model,\n")
	fmt.Printf("reduce the model, save intermediate files, but not solve the problem.\n\n")
	
	// In a previous incarnation, all values were provided by the user.
	// Now they are hard-coded, so populate the control structure directly.

	psCtrl.DelRowNonbinding  = true
	psCtrl.DelRowSingleton   = true
	psCtrl.DelColSingleton   = true
	psCtrl.DelFixedVars      = true
	psCtrl.RunSolver         = false
	psCtrl.MaxIter           = 10
	psCtrl.FileInMps         = ""
	psCtrl.FileOutSoln       = ""
	psCtrl.FileOutPsop       = ""
	psCtrl.FileOutMpsRdcd    = ""

	// Execute the various functions to read the MPS file, print the model,
	// print the statistics, reduce the matrix, print the reduced matrix to an
	// MPS file, and print a file of the pre-solve operations (PSOP).
	if err = lpo.ReadMpsFile(inputSmLP); err != nil {
		return errors.Wrap(err, "wpShowProb failed reading MPS file")
	}

	if err = lpo.PrintModel(); err != nil {
		return errors.Wrap(err, "wpShowProb failed to print model")		
	}

	fmt.Printf("\nPaused after lpo.PrintModel...\n")
	if err = wpPauseOutput(); err != nil {
		return errors.Wrap(err, "wpShowProb aborted")				
	}
		
	if err = lpo.GetStatistics(&lpStats); err != nil {
		return errors.Wrap(err, "wpShowProb failed to get statistics")		
	}			

	if err = lpo.PrintStatistics(lpStats); err != nil {
		return errors.Wrap(err, "wpShowProb failed to print statistics")		
	}			

	fmt.Printf("\nPaused after lpo.PrintStatistics...\n")
	if err = wpPauseOutput(); err != nil {
		return errors.Wrap(err, "wpShowProb aborted")				
	}

	if err = lpo.ReduceMatrix(psCtrl); err != nil {
		return errors.Wrap(err, "wpShowProb failed to reduce matrix")		
	}			

	if err = lpo.WriteMpsFile(outRedMtx); err != nil {
		return errors.Wrap(err, "wpShowProb failed to write reduce matrix file")		
	}			
		
	if err = lpo.WritePsopFile(outPsop, 2); err != nil {
		return errors.Wrap(err, "wpShowProb failed to write PSOP file")		
	}			
		
	return nil
}

//==============================================================================

// wpCoinSolveProb illustrates an example of a problem solved using the internal
// data structures. It reads data from file, populates the internal data structures,
// solves the problem, prints the solution, and gives user the option to save
// the model and solution to file.
// The function accepts the MPS file name defining the model as input.
// In case of failure, function returns an error.
func wpCoinSolveProb(fileName string) error {
	var userString      string  // holder for general input from user
	var psCtrl      lpo.PsCtrl  // control structure for reductions
	var err              error  // error received from called functions

	fmt.Printf("\nThis example illustrates how to read the model definition from an\n")
	fmt.Printf("MPS file, reduce the problem size, solve it via Coin-OR, and display\n")
	fmt.Printf("the results.\n\n")
	
	// In a previous incarnation, all values were provided by the user.
	// Now they are hard-coded, so populate the control structure directly.

	psCtrl.DelRowNonbinding  = true
	psCtrl.DelRowSingleton   = true
	psCtrl.DelColSingleton   = true
	psCtrl.DelFixedVars      = true
	psCtrl.RunSolver         = true
	psCtrl.MaxIter           = 10
	psCtrl.FileInMps         = fileName
	psCtrl.FileOutSoln       = outSoln
	psCtrl.FileOutPsop       = outPsop
	psCtrl.FileOutMpsRdcd    = outRedMtx

	startTime := time.Now()					
	err = lpo.CoinSolveProb(psCtrl, &psResult)
	endTime := time.Now()
			
	if err != nil {
		return errors.Wrap(err, "wpCplexSolveProb failed")
	} else {
		fmt.Printf("\nOBJECTIVE FUNCTION = %f\n\n", psResult.ObjVal)
		fmt.Printf("Presolve removed %d rows, %d cols, and %d elements.\n",
			psResult.RowsDel, psResult.ColsDel, psResult.ElemDel)
		fmt.Printf("Solution has %d constraints and %d variables.\n", 
			len(psResult.ConMap), len(psResult.VarMap))

		// Display which files were used.			
		if psCtrl.FileInMps != "" {
			fmt.Printf("Input MPS file read:    '%s'\n", psCtrl.FileInMps)
		} else {
			fmt.Printf("Model read from internal data structures.\n")
		}
		
		if psCtrl.FileOutSoln != "" {
			fmt.Printf("Solution file saved:    '%s'\n", psCtrl.FileOutSoln)
		}

		if psCtrl.FileOutMpsRdcd != "" {
			fmt.Printf("Reduced MPS file saved: '%s'\n", psCtrl.FileOutMpsRdcd)
		}

		if psCtrl.FileOutPsop != "" {
			fmt.Printf("PSOP file saved:        '%s'\n", psCtrl.FileOutPsop)
		}
		
		fmt.Printf("\nStarted at:  %s\n",   startTime.Format("2006-01-02 15:04:05"))
		fmt.Printf("Finished at: %s\n\n", endTime.Format("2006-01-02 15:04:05"))

		fmt.Printf("Do you want to see the detailed solution [Y|N]: ")
		fmt.Scanln(&userString)
		if userString == "y" || userString == "Y" {
			wpPrintLpoSoln()			
		}

	} // End else there was no error
		
	return nil
}


//==============================================================================

// runMainWrapper displays the menu of options available, prompts the user to enter
// one of the options, and executes the command specified. The main wrapper controls
// the main commands, and in turn calls secondary wrappers to execute additional
// commands. The flags which control the display of menu options have no impact on
// the available commands. All commands are available even if the corresponding menu
// item is "hidden". The function accepts no arguments and returns no values.
func runMainWrapper() {
	var cmdOption     string  // command option
	var err            error  // error returned by called functions

	// Print header and enter infinite loop until user quits.

	fmt.Println("\nDEMONSTRATION OF LPO FUNCTIONALITY.")
	
	for {

		// Initialize variables, read command, and execute command.		
		printOptions()
		cmdOption    = ""		
		fmt.Printf("\nEnter a new option: ")
		fmt.Scanln(&cmdOption)

		switch cmdOption {

		case "0":
			fmt.Println("\n===> NORMAL PROGRAM TERMINATION <===\n")
			return

		case "1":
			// Load and show problem but don't solve.
			if err = wpShowProb(); err != nil {
				fmt.Println(err)
			}

		case "2":
			// Solve small LP using Coin-OR CLP.
			if err = wpCoinSolveProb(inputSmLP); err != nil {
				fmt.Println(err)
			}
		
		case "3":
			// Solve small MILP using Coin-OR CBC.
			if err = wpCoinSolveProb(inputSmMILP); err != nil {
				fmt.Println(err)
			}
		
		case "4":
			err = nil
			
			// Comment out the following line if gpx is not installed and
			// the utilsgpx.go file is excluded from being built.
			err = wpCplexSolveProb()
			
			if err != nil {
				fmt.Println(err)
			}
								
		case "5":
			wpPrintLpoSoln()
												
		default:
			fmt.Printf("Unsupported option: '%s'\n", cmdOption)
						
		} // end of switch on cmdOption
	} // end for looping over commands

}

//==============================================================================

// main function calls the main wrapper. It accepts no arguments and returns
// no values.
func main() {
	
	runMainWrapper()
}

//============================ END OF FILE =====================================