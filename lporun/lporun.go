// This file contains the main function and functions called from the main menu.

// KLUDGE ALERT: Since the exerciser needs to pass information to functions in a
// sequence that would not generally be used by a "real" program, several arguments
// have been declared as global variables within the scope of this package, and may,
// or may not, be passed via function arguments, depending on the function.

package main

import (
	"fmt"
	"github.com/Beldin123/lpo"
	"github.com/pkg/errors"
	"os"
	"time"
)

// Flags to control display of menus and use of customized environment.

var mainMenuOn bool = true    // Flag for main lpo function display
var lpoMenuOn  bool = false   // Flag for enabling lpo functions   
var gpxMenuOn  bool = false   // Flag for enabling gpx functions   
var custEnvOn  bool = true    // Flag for enabling custom paths and names
var pauseAfter int  = 50      // Number of items to print before pausing

// Customized environment used if custEnvOn = true.
// It is intended to reduce the amount of typing for SOME (not all) user input,
// and to build names of internal files related to the "base" name specified.
// If disabled, user must enter complete directory and file names when prompted.

var dSrcDev       string = "D:/Docs/LP/Data/"           // Development source data dir
var fPrefCplexOut string = "cp_"    // Prefix for Cplex solution xml files  
var fPrefRdcMps   string = "r_"     // Prefix for MPS file storing reduced matrix
var fPrefPsopOut  string = "psop_"  // Prefix for file storing data removed during PSOP
var fExtension    string = ".txt"   // Extension of source data files in development dir.  

// Need to declare lpo variables here to avoid passing them as arguments to the
// wrapper functions as individual wrapper commands are executed.

var lpCpSoln lpo.CplexSoln    // Cplex solution obtained from parsing xml file
var lpStats  lpo.Statistics   // statistics data structure
var psResult lpo.PsSoln       // solution received from lpo

// Delimiter for sections in GPX input file

const fileDelim = "#------------------------------------------------------------------------------\n"

//==============================================================================

// printOptions displays the options that are available for testing. Package
// global flags control which menus are printed.
// The function accepts no arguments and returns no values.
func printOptions() {

	fmt.Println("\nAvailable Options (0 to EXIT):")
	fmt.Println("")
	fmt.Println(" m - toggle main menu display                c - toggle custom environment")
	fmt.Println(" s - toggle lpo function exerciser           g - toggle gpx function exerciser")
	
  if mainMenuOn {
	fmt.Println("")
	fmt.Println(" 1 - using SolveProb   2 - using Cplex func  3 - using ReduceMtrx  4 - read MPS file")
	fmt.Println(" 5 - write MPS file    6 - init. lpo struct  7 - show lpo input    8 - show  lpo soln.")
	fmt.Println(" 9 - init. gpx struct 10 - write gpx file   11 - show gpx input   12 - show  gpx soln.")
	fmt.Println("13 - show Cplex soln")
  }

  if lpoMenuOn {
	fmt.Println("")
	fmt.Println("21 - AdjustModel      22 - CalcConViolation 23 - CalcLhs          24 - CplexCreateProb")
	fmt.Println("25 - CplexParseSoln   26 - CplexSolveMps    27 - DelCol           28 - DelRow")
	fmt.Println("29 - GetLogLevel      30 - GetStatistics    31 - GetTempDirPath   32 - InitModel")
	fmt.Println("33 - PrintCol         34 - PrintModel       35 - PrintRhs         36 - PrintRow")
	fmt.Println("37 - PrintStatistics  38 - ReadMpsFile      39 - ReduceMatrix     40 - ScaleRows")
	fmt.Println("41 - SetLogLevel      42 - SetTempDirPath   43 - SolveProb        44 - TightenBounds")
	fmt.Println("45 - TransFromGpx     46 - TransToGpx       47 - WriteMpsFile     48 - WritePsopFile")
  }

  if gpxMenuOn {
	fmt.Println("")
	fmt.Println("61 - ChgCoefList      62 - ChgObjSen        63 - ChgProbName      64 - CloseCplex")
	fmt.Println("65 - CreateProb       66 - GetColName       67 - GetMipSolution   68 - GetNumCols")
	fmt.Println("69 - GetNumRows       70 - GetObjVal        71 - GetRowName       72 - GetSlack")
	fmt.Println("73 - GetSolution      74 - GetX             75 - LpOpt            76 - MipOpt")
	fmt.Println("77 - NewCols          78 - NewRows          79 - OutputToScreen   80 - ReadCopyProb")
	fmt.Println("81 - SolWrite         82 - WriteProb")
  }

}

//==============================================================================

// wpInitLpo initializes all input, solution, and other (e.g. statistics) data
// structures. As much as possible, it uses the initialization routines from the
// lpo package. The function accepts no arguments.
// In case of failure, function returns an error.
func wpInitLpo() {

	lpo.InitModel()

	// If the model is empty, and we get the statistics, we actually initialize
	// the statistics data structure (since there is nothing to get).
	
	lpo.GetStatistics(&lpStats)

	// Similarly, if we call CplexParseSoln with a bogus file name and ignore the
	// error, we get back the initialized Cplex solution data structure (again
	// because there is nothing to get.
	
	_ = lpo.CplexParseSoln("", &lpCpSoln)
				
	// The only thing left to initialize is the solution data structure.
		
    psResult.ColsDel = 0
	psResult.RowsDel = 0
	psResult.ElemDel = 0
	psResult.ObjVal  = 0.0
	psResult.ConMap  = nil
	psResult.VarMap  = nil


	fmt.Printf("All lpo data structures have been initialized.\n")
		
}

//==============================================================================

// wpSolveProb illustrates an example of a problem solved using the internal
// data structures. It reads data from file, populates the internal data structures,
// solves the problem, prints the solution, and gives user the option to save
// the model and solution to file. Function accepts no arguments.
// In case of failure, function returns an error.
func wpSolveProb() error {
	var fileNameMPS         string  // MPS input file for the model
	var filePsopOut         string  // output file for pre-solve reductions
	var fileCplexOut        string  // output file for Cplex xml solution
	var flagChoice          string  // flag selection read from user
	var userString          string  // holder for general input from user
	var runTB, runRowS        bool  // flags for row reductions
	var runColS, runFixedVars bool  // flags for column reductions
	var runCplex              bool  // flag controlling if problem is solved
	var psCtrl          lpo.PsCtrl  // control structure for reductions
	var err                  error  // error received from called functions


	fmt.Printf("\nThis example illustrates how to read the model definition from an\n")
	fmt.Printf("MPS file, reduce the problem size, solve it via Cplex, and display\n")
	fmt.Printf("the results. The only exported function called is lpo.SolveProb.\n")
	fmt.Printf("All user input requested is to populate the lpo.PsCtrl data structure.\n\n")
	
	
	// Initialize file names to be empty string.
	fileNameMPS  = ""
	fileCplexOut = ""
	filePsopOut  = ""

	// Enter input and output file names.	
	fmt.Printf("Enter MPS input file name or <CR> to use data structures: ")
	fmt.Scanln(&fileNameMPS)

	if fileNameMPS == "" {
		if len(lpo.Rows) == 0 || len(lpo.Cols) == 0 || len(lpo.Elems) == 0 {
			fmt.Printf("ERROR: At least one internal data structure is empty.\n")
			return errors.New("wpSolveProb failed, model not defined")
		}
	}	

	if custEnvOn {
		// If input file was specified, set output file correspondingly.
		// Otherwise input will be from data structures, and output will be default.
		if fileNameMPS != "" {
			// Create base name using input MPS file and tack on the right prefix.
			fileCplexOut = fPrefCplexOut + fileNameMPS
			filePsopOut  = fPrefPsopOut  + fileNameMPS
			// Add the full directory path and extension.
			fileNameMPS  = dSrcDev + fileNameMPS  + fExtension			
			fileCplexOut = dSrcDev + fileCplexOut + fExtension
			filePsopOut  = dSrcDev + filePsopOut  + fExtension
		}
	} else {
		fmt.Printf("Enter Cplex output file name or <CR> for none: ")
		fmt.Scanln(&fileCplexOut)		
		fmt.Printf("Enter PSOP output file name or <CR> for none: ")
		fmt.Scanln(&filePsopOut)		
	}

	// Initialize and set the problem reduction flags.
	runTB        = false
	runRowS      = false
	runColS      = false
	runFixedVars = false
	runCplex     = true			

	fmt.Printf("Do you want the problem reduced and solved ['all' | 'none' | <CR> to set]: ")
	fmt.Scanln(&flagChoice)
		
	if flagChoice == "all" {
		runTB        = true
		runRowS      = true
		runColS      = true
		runFixedVars = true
	} else if flagChoice == "none" {
		// Default state, no changes.
	} else {
		userString = ""
		fmt.Printf("Do you wish to run TightenBounts [Y|N]: ")
		fmt.Scanln(&userString)
		if userString == "y" || userString == "Y" {
			runTB = true
		}
		
		userString = ""
		fmt.Printf("Do you wish to remove row singletons [Y|N]: ")
		fmt.Scanln(&userString)
		if userString == "y" || userString == "Y" {
			runRowS = true
		}

		userString = ""
		fmt.Printf("Do you wish to remove column singletons [Y|N]: ")
		fmt.Scanln(&userString)
		if userString == "y" || userString == "Y" {
			runColS = true
		}

		userString = ""
		fmt.Printf("Do you wish to remove fixed variables [Y|N]: ")
		fmt.Scanln(&userString)
		if userString == "y" || userString == "Y" {
			runFixedVars = true
		}

		userString = ""
		fmt.Printf("Do you wish solve the problem via Cplex [Y|N]: ")
		fmt.Scanln(&userString)
		if userString == "y" || userString == "Y" {
			// Default state
		} else {
			runCplex = false
		}
		
	} // end else setting individual flags

	psCtrl.DelRowNonbinding  = runTB
	psCtrl.DelRowSingleton   = runRowS
	psCtrl.DelColSingleton   = runColS
	psCtrl.DelFixedVars      = runFixedVars
	psCtrl.RunCplex          = runCplex
	psCtrl.MaxIter           = 10
	psCtrl.FileInMps         = fileNameMPS
	psCtrl.FileOutCplexSoln  = fileCplexOut
	psCtrl.FileOutPsop       = filePsopOut

	startTime := time.Now()					
	err = lpo.SolveProb(psCtrl, &psResult)
	endTime := time.Now()
			
	if err != nil {
		return errors.Wrap(err, "wpSolveProb failed")
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
		
		if psCtrl.FileOutCplexSoln != "" {
			fmt.Printf("Cplex solution saved:   '%s'\n", psCtrl.FileOutCplexSoln)
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

// wpSolveCplex is a wrapper obtaining a solution directly from an MPS data file.
// During execution, the user provides the name of the source MPS file (base name if
// customEnv is true, full path otherwise). The function sets up the Cplex control
// file, instructs Cplex to read the file and solve it using function lpo.SolveCplexMps.
// The solution xml file is parsed using lpo.CplexParseSoln. The value of the
// objective function as well as the names of all files are displayed to the user.
// If needed, the raw xml output file can be checked in an editor, or via other
// functions provided in this module.
// The function accepts no arguments and returns no values.
func wpSolveCplex() error {
	var userString     string  // user input string
	var fileName       string  // MPS input file
	var filePresolve   string  // presolve file used by Cplex
	var fileCplexOut   string  // output file generated by Cplex
	var err             error  // error received from called functions

	// Get the name of the source MPS file and generate other file names from the
	// base name, or prompt user for them if custom environment is disabled.

	fmt.Printf("\nThis example illustrates how to read an MPS file directly by\n")
	fmt.Printf("Cplex (without using lpo or gpx data structures), solve the problem,\n")
	fmt.Printf("and display the results by parsing the Cplex solution file.\n")
	fmt.Printf("The functions used are lpo.CplexSolveMps and lpo.CplexParseSoln.\n\n")

		
	fmt.Printf("\nEnter MPS file to be read by cplex: ")
	fmt.Scanln(&fileName)
	if custEnvOn {
		filePresolve = ""
		fileCplexOut = dSrcDev + fPrefCplexOut + fileName + fExtension
		fileName     = dSrcDev +                 fileName + fExtension
	} else {
		fmt.Printf("Enter cplex output file: ")
		fmt.Scanln(&fileCplexOut)
		fmt.Printf("Enter presolve file: ")
		fmt.Scanln(&filePresolve)
	}

	// Call the functions to solve the problem, parse the solution, and display
	// the results.

	fmt.Println("")	
	
	err = lpo.CplexSolveMps(fileName, fileCplexOut, filePresolve, &lpCpSoln)
	if err != nil {
		return errors.Wrap(err, "wpSolveCplex failed solving problem")			
	}
						
	if err = lpo.CplexParseSoln(fileCplexOut, &lpCpSoln); err != nil {
		return errors.Wrap(err, "wpSolveCplex failed parsing solution")			
	}
	
	fmt.Printf("\nMPS file read:      %s\n", fileName)
	fmt.Printf("Cplex output:       %s\n", fileCplexOut)
	fmt.Printf("Presolve file:      %s\n", filePresolve)
	fmt.Printf("Objective value:    %f\n\n", lpCpSoln.Header.ObjValue)						

	userString = ""
	fmt.Printf("Display Cplex solution [Y|N]: ")
	fmt.Scanln(&userString)
	if userString == "y" || userString == "Y" {
		wpPrintCplexSoln()
	}

	return nil
}

//==============================================================================

// wpPrintCplexSoln prints the solution generated by cplex and written to xml file.
// Function uses the parsed Cplex output contained in the global variable. 
// It returns nothing.
func wpPrintCplexSoln() {
	var userString string
	var counter    int
		
	fmt.Println("\nSolution from cplex:\n")

	fmt.Println("Version:        ", lpCpSoln.Version)
	fmt.Println("ProblemName:    ", lpCpSoln.Header.ProblemName)
	fmt.Println("ObjValue:       ", lpCpSoln.Header.ObjValue)
	fmt.Println("SolTypeValue:   ", lpCpSoln.Header.SolTypeValue)
	fmt.Println("SolTypeString:  ", lpCpSoln.Header.SolTypeString)
	fmt.Println("SolStatusValue: ", lpCpSoln.Header.SolStatusValue)
	fmt.Println("SolStatusString:", lpCpSoln.Header.SolStatusString)
	fmt.Println("SolMethodString:", lpCpSoln.Header.SolMethodString)
	fmt.Println("PrimalFeasible: ", lpCpSoln.Header.PrimalFeasible)
	fmt.Println("DualFeasuble:   ", lpCpSoln.Header.DualFeasible)
	fmt.Println("SimplexItns:    ", lpCpSoln.Header.SimplexItns)
	fmt.Println("BarrierItns:    ", lpCpSoln.Header.BarrierItns)
	fmt.Println("WriteLevel:     ", lpCpSoln.Header.WriteLevel)
	fmt.Println("EpRHS:          ", lpCpSoln.Quality.EpRHS)
	fmt.Println("EpOpt:          ", lpCpSoln.Quality.EpOpt)
	fmt.Println("MaxPrimalInfeas:", lpCpSoln.Quality.MaxPrimalInfeas)
	fmt.Println("MaxDualInfeas:  ", lpCpSoln.Quality.MaxDualInfeas)
	fmt.Println("MaxPrimalResid: ", lpCpSoln.Quality.MaxPrimalResidual)
	fmt.Println("MaxDualResidual:", lpCpSoln.Quality.MaxDualResidual)
	fmt.Println("Quality.MaxX:   ", lpCpSoln.Quality.MaxX)
	fmt.Println("Quality.MaxPi:  ", lpCpSoln.Quality.MaxPi)
	fmt.Println("Qual.MaxSlack:  ", lpCpSoln.Quality.MaxSlack)
	fmt.Println("Qual.MaxRedCost:", lpCpSoln.Quality.MaxRedCost)
	fmt.Println("Quality.Kappa:  ", lpCpSoln.Quality.Kappa)
	
	userString = ""
	counter    = 0
	fmt.Printf("\nDisplay variables list [Y|N]: ")
	fmt.Scanln(&userString)
	if userString == "y" || userString == "Y" {
		for i := 0; i < len (lpCpSoln.Varbs); i++ {
			fmt.Printf("%4d: ", i)
			fmt.Println(lpCpSoln.Varbs[i])
			counter++
			if counter == pauseAfter {
				counter = 0
				userString = ""
				fmt.Printf("\nPAUSED... <CR> continue, any key to quit: ")
				fmt.Scanln(&userString)
				if userString != "" {
					break 
				}
			} // end if pause required
		}
	}

	userString = ""
	counter    = 0
	fmt.Printf("\nDisplay constraints list [Y|N]: ")
	fmt.Scanln(&userString)
	if userString == "y" || userString == "Y" {
		for i := 0; i < len (lpCpSoln.LinCons); i++ {
			fmt.Printf("%4d: ", i)
			fmt.Println(lpCpSoln.LinCons[i])
			counter++
			if counter == pauseAfter {
				counter = 0
				userString = ""
				fmt.Printf("\nPAUSED... <CR> continue, any key to quit: ")
				fmt.Scanln(&userString)
				if userString != "" {
					break 
				}
			} // end if pause required
		} // end for constraints list
	} // end if printing constraints

}
//==============================================================================

// wpReduceMtrx is a wrapper for lpo.ReduceMatrix. During execution, it prompts
// the user for all relevant parameters needed to populate the control structure
// and calls the ReduceMatrix function. It assumes that other functions are used
// to populate the model and process the results.
// The function accepts no arguments and returns no values.
func wpReduceMtrx() error {

	var psCtrl lpo.PsCtrl   // pre-solve control structure
	var flagChoice string   // choice of which options to select
	var userString string   // input provided by user
	var runTB        bool   // run TightenBounds
	var runRowS      bool   // remove row singletons
	var runColS      bool   // remove column singletons
	var runFixedVars bool   // remove fixed variables
	var err         error   // error returned from called functions

	// Initialize the variables, which also become the "none" option provided
	// by the user.
	flagChoice   = ""
	runTB        = false
	runRowS      = false
	runColS      = false
	runFixedVars = false

	// Get the options from the user, and change flags as needed.
	fmt.Printf("SolveProb flags ('all' | 'none' | <CR> to set): ")
	fmt.Scanln(&flagChoice)
	
	if flagChoice == "all" {
		runTB        = true
		runRowS      = true
		runColS      = true
		runFixedVars = true
	} else if flagChoice == "none" {
		// Default state
	} else {
		userString = ""
		fmt.Printf("Do you wish to run TightenBounts [Y|N]: ")
		fmt.Scanln(&userString)
		if userString == "y" || userString == "Y" {
			runTB = true
		}
		
		userString = ""
		fmt.Printf("Do you wish to remove row singletons [Y|N]: ")
		fmt.Scanln(&userString)
		if userString == "y" || userString == "Y" {
			runRowS = true
		}

		userString = ""
		fmt.Printf("Do you wish to remove column singletons [Y|N]: ")
		fmt.Scanln(&userString)
		if userString == "y" || userString == "Y" {
			runColS = true
		}

		userString = ""
		fmt.Printf("Do you wish to remove fixed variables [Y|N]: ")
		fmt.Scanln(&userString)
		if userString == "y" || userString == "Y" {
			runFixedVars = true
		}
				
	} // end else setting reduction flags

	// Populate the control data structure and call ReduceMatrix.	
	psCtrl.DelRowNonbinding = runTB
	psCtrl.DelRowSingleton  = runRowS
	psCtrl.DelColSingleton  = runColS
	psCtrl.DelFixedVars     = runFixedVars
	psCtrl.RunCplex         = false
	psCtrl.MaxIter          = 20
	psCtrl.FileInMps        = ""
	psCtrl.FileOutCplexSoln = ""

	if err = lpo.ReduceMatrix(psCtrl); err != nil {
		return errors.Wrap(err, "wpReduceMtrx failed")
	}
	
	return nil
}

//==============================================================================

// wpPrintLpoIn prints the input data structures in their raw format, directly
// as entries in the appropriate list. The function accepts no arguments and returns
// no values.
func wpPrintLpoIn() {
	var userString string  // user input
	var counter    int     // counter keeping track of number of lines printed

	if lpo.Name != "" {
		fmt.Printf("Problem [%s], obj. index %d\n", lpo.Name, lpo.ObjRow)
	} else {
		fmt.Printf("WARNING: Problem name is empty.\n")
	}

	// If the Rows list is not empty, and the user wants to see it, print it.
	if len(lpo.Rows) != 0 {
		userString = ""
		fmt.Printf("\nDisplay rows list [Y|N]: ")
		fmt.Scanln(&userString)
		if userString == "y" || userString == "Y" {
			counter = 0
			fmt.Printf("%d rows are:\n", len(lpo.Rows))

			for i := 0; i < len(lpo.Rows); i++ {
				fmt.Println(i, lpo.Rows[i])
				counter++
				if counter == pauseAfter {
					counter = 0
					userString = ""
					fmt.Printf("\nPAUSED... <CR> continue, any key to quit: ")
					fmt.Scanln(&userString)
					if userString != "" {
						break 
					} // End if quitting print statement					
				} // End if pause required
			} // End for all rows
		} // end if displaying list		
	} else {
		fmt.Printf("WARNING: Rows list is empty.\n")
	}	

	// If the Cols list is not empty, and the user wants to see it, print it.
	if len(lpo.Cols) != 0 {
		userString = ""
		fmt.Printf("\nDisplay columns list [Y|N]: ")
		fmt.Scanln(&userString)
		if userString == "y" || userString == "Y" {
			counter = 0
			fmt.Printf("%d columns are:\n", len(lpo.Cols))
			for i := 0; i < len(lpo.Cols); i++ {
				fmt.Println(i, lpo.Cols[i])
				counter++
				if counter == pauseAfter {
					counter = 0
					userString = ""
					fmt.Printf("\nPAUSED... <CR> continue, any key to quit: ")
					fmt.Scanln(&userString)
					if userString != "" {
						break 
					}
				} // End if pause required
			} // End for all columns
		} // end if displaying list		
	} else {
		fmt.Printf("WARNING: Columns list is empty.\n")
	}	

	// If the Elems list is not empty and the user wants to see it, print it.
	if len(lpo.Elems) != 0 {
		userString = ""
		fmt.Printf("\nDisplay elements list [Y|N]: ")
		fmt.Scanln(&userString)
		if userString == "y" || userString == "Y" {
			counter = 0
			fmt.Printf("%d elements are:\n", len(lpo.Elems))
			for i := 0; i < len(lpo.Elems); i++ {
				fmt.Println(i, lpo.Elems[i])
				counter++
				if counter == pauseAfter {
					counter = 0
					userString = ""
					fmt.Printf("\nPAUSED... <CR> continue, any key to quit: ")
					fmt.Scanln(&userString)
					if userString != "" {
						break 
					}
				} // end if pause required
			} // end for all elements
		} // end if displaying list
	} else {
		fmt.Printf("WARNING: Elements list is empty.\n")
	}	
	
}

//==============================================================================

// wpPrintLpoSoln prints the solution contained in the lpo data structures. It
// presents the data in a formatted manner, and gives the user the option to pause
// periodically so output does not scroll off the screen. The function accepts no
// input and returns no values.
func wpPrintLpoSoln() {
	var userString string
	var counter int

	// Check if the lists exist, and if they do, print them.
					
	if len(psResult.VarMap)	<= 0 {
		fmt.Printf("WARNING: Solution list of variables is empty.\n")
	} else {
		userString = ""
		fmt.Printf("\nDisplay variable list [Y|N]: ")
		fmt.Scanln(&userString)
		if userString == "y" || userString == "Y" {
			fmt.Printf("Variables are:\n")
			fmt.Printf("  %-10s %-4s     %15s %15s %15s\n", "NAME", "ST", "VALUE", 
				"REDUCED COST", "SCALE FACTOR")
			
			counter = 0
			for psVarbName, psVarb := range psResult.VarMap {
				fmt.Printf("  %-10s %-4s     %15e %15e %15e\n", psVarbName, psVarb.Status,
					psVarb.Value, psVarb.ReducedCost, psVarb.ScaleFactor)
					
				counter++
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
			fmt.Printf("  %-10s %-4s %3s %15s %15s %15s %15s\n", "ROW",
					"ST", "EQ", "RHS", "SLACK", "PI", "SCALE FACTOR")
				
			counter = 0
			for psConName,psCon := range psResult.ConMap {
				fmt.Printf("  %-10s %-4s %3s %15e %15e %15e %15e\n",
					psConName, psCon.Status, psCon.Type,
					psCon.Rhs, psCon.Slack, psCon.Pi, psCon.ScaleFactor)
				counter++
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

// wpWriteGpx takes the model contained in the lpo structures, translates them to
// the gpx data structures, and prints the contents of the gpx data structures in
// a text file, which can be read at a later time by the gpxrun executable. The
// intent of this round-about mechanism is to transfer lpo data to gpx, which cannot
// import any lpo data structures or functions. This function is intended purely for
// the tutorial and is not needed by the main gpx package. The function accepts 
// no arguments. In case of failure, the function returns an error.
func wpWriteGpx() error {
	var fileName string   // name of file to which gpx data are written
	var err      error    // error returned from functions called


	// Prompt the user for the name of the file, and adjust if custom environment
	// is enabled.

	fmt.Printf("Enter name of GPX file to be written: ")
	fmt.Scanln(&fileName)
	if custEnvOn {
		fileName = dSrcDev + fileName + fExtension
	}

	//Check whether the file exists. If it exists, overwrite it.

	if _, err := os.Stat(fileName); err == nil {
		err = os.Remove(fileName)
		if err != nil {
			return errors.Wrapf(err, "Failed to delete existing file %s", fileName)
		}
	}
	
	f, err := os.Create(fileName)
	if err != nil {
		return errors.Wrapf(err, "Failed to create new file %s", fileName)
	}

 	defer f.Close()

	err = lpo.TransToGpx(&gRows, &gCols, &gElem, &gObj)
	if err != nil {
		return errors.Wrap(err, "Failed to translate from LPO to GPX")		
	} 

	// Print the file header
	startTime := time.Now()

	fmt.Fprintf(f, "%s", fileDelim)
	fmt.Fprintf(f, "# GPX input data file\n")	
	fmt.Fprintf(f, "# Created on:   %s\n", startTime.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(f, "PROBLEM_NAME: %s\n", lpo.Name)

	// Print the objective function
	fmt.Fprintf(f, "%s", fileDelim)
	fmt.Fprintf(f, "# objective_coef_index objective_coef_value\n")
	fmt.Fprintf(f, "OBJECTIVE_START\n")
	for i := 0; i < len(gObj); i++ {
		fmt.Fprintf(f, "%d %f\n", gObj[i].ColIndex, gObj[i].Value)
	}

	// Print the rows		
	fmt.Fprintf(f, "%s", fileDelim)
	fmt.Fprintf(f, "# row_name row_sense row_rhs row_rngval\n")
	fmt.Fprintf(f, "ROWS_START\n")
	for i := 0; i < len(gRows); i++ {
		fmt.Fprintf(f, "%s %s %f %f\n", gRows[i].Name, gRows[i].Sense, gRows[i].Rhs, gRows[i].RngVal)
	}

	// Print the columns
	fmt.Fprintf(f, "%s", fileDelim)
	fmt.Fprintf(f, "# col_name col_type col_lower_bound col_upper_bound\n")
	fmt.Fprintf(f, "COLUMNS_START\n")
	for i := 0; i < len(gCols); i++ {
		fmt.Fprintf(f, "%s %s %f %f\n", gCols[i].Name, gCols[i].Type, gCols[i].BndLo, gCols[i].BndUp)
	}
	
	// Print the non-zero elements
	fmt.Fprintf(f, "%s", fileDelim)
	fmt.Fprintf(f, "# elem_in_row_index elem_in_col_index elem_value\n")
	fmt.Fprintf(f, "ELEMENTS_START\n")
	for i := 0; i < len(gElem); i++ {
		fmt.Fprintf(f, "%d %d %f\n", gElem[i].RowIndex, gElem[i].ColIndex, gElem[i].Value)
	}

	fmt.Fprintf(f, "END_DATA\n")
	
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

	var fileName      string  // file name
	var cmdOption     string  // command option
	var err            error  // error returned by called functions


	// Print header and options, and enter infinite loop until user quits.

	fmt.Println("\nTUTORIAL AND EXERCISER FOR LPO AND GPX FUNCTIONS.")
	printOptions()
	
	for {

		// Initialize variables, read command, and execute command.
		
		cmdOption    = ""		
		fmt.Printf("\nEnter a new option: ")
		fmt.Scanln(&cmdOption)

		switch cmdOption {

		//---------------- Commands for toggles --------------------------------

		case "m":
			if mainMenuOn {
				mainMenuOn = false
				fmt.Println("\nMain menu will be hidden.")
			} else {
				mainMenuOn = true
				fmt.Println("\nMain menu will be displayed.")
				printOptions()				
			}

		case "s":
			if lpoMenuOn {
				lpoMenuOn = false
				fmt.Println("\nLPO functions menu commands will be disabled.")
			} else {
				lpoMenuOn = true
				fmt.Println("\nLPO functions menu commands will be enabled.")
				printOptions()				
			}

		case "g":
			if gpxMenuOn {
				gpxMenuOn = false
				fmt.Println("\nGPX functions menu commands will be disabled.")
			} else {
				gpxMenuOn = true
				fmt.Println("\nGPX functions menu commands will be enabled.")
				printOptions()				
			}
						
		case "c":
			if custEnvOn {
				fmt.Printf("\nCustomized environment disabled.\n")
				fmt.Printf("Full file paths must be entered when needed.\n")
				custEnvOn = false
			} else {
				fmt.Printf("\nWARNING: Customized environment enabled.\n\n")
				fmt.Printf("When prompted for file names, only the base name (without path,\n")
				fmt.Printf("file prefix, or file extension) needs to be entered.\n\n")
				fmt.Printf("Directory for all files          = '%s'\n", dSrcDev)
				fmt.Printf("Extension for all files          = '%s'\n", fExtension)
				fmt.Printf("Prefix for Cplex output          = '%s'\n", fPrefCplexOut)
				fmt.Printf("Prefix for reduced matrix output = '%s'\n", fPrefRdcMps)
				fmt.Printf("Prefix for post-solve operations = '%s'\n", fPrefPsopOut)
				custEnvOn = true				
			}

		//------------- Functions exercised by main wrapper --------------------

		case "0":
			fmt.Println("\n===> NORMAL PROGRAM TERMINATION <===\n")
			return


		case "1", "43":
			// Solve problem from internal structures
			err = wpSolveProb()
			if err != nil {
				fmt.Println(err)
			} else {
				fmt.Printf("\nExample showing SolveProb completed successfully.\n")
			}

		case "2", "26":
			// Read and solve MPS file directly by Cplex
			err = wpSolveCplex()
			if err != nil {
				fmt.Println(err)
			} else {
				fmt.Printf("\nExample using Cplex directly completed successfully.\n")
			}

		case "3", "39":
			// ReduceMatrix
			err = wpReduceMtrx()
			if err != nil {
				fmt.Println(err)
			} else {
				fmt.Printf("\nExample showing ReduceMatrix completed successfully.\n")
			}

		case "4", "38":
			// Read MPS file
			fmt.Printf("Enter name of MPS file to be read: ")
			fmt.Scanln(&fileName)
			if custEnvOn {
				fileName = dSrcDev + fileName + fExtension
			}
			fmt.Println("Reading file", fileName)
			if err = lpo.ReadMpsFile(fileName); err != nil {
				fmt.Println(err)
			}

		case "5", "47":
			// Write MPS file
			fmt.Printf("Enter MPS output file name: ")
			fmt.Scanln(&fileName)
			if custEnvOn {
				fileName = dSrcDev + fileName + fExtension
			}
			err = lpo.WriteMpsFile(fileName)
			if err != nil {
				fmt.Println(err)
			} else {
				fmt.Printf("Model successfully written to file '%s'.\n", fileName)
			}

		case "6":
			wpInitLpo()	
			
		case "7":
			// Print LPO input data structures
			wpPrintLpoIn()
			
		case "8":
			// Print LPO solution data structures
			wpPrintLpoSoln()	

		case "9":
			// Initialize GPX data structures
			wpInitGpx()
			
		case "10":
			// Write GPX input file
			err = wpWriteGpx()
			if err != nil {
				fmt.Println(err)
			} else {
				fmt.Printf("GPX data file written successfully.\n")				
			}
		
		case "11":
			// Print GPX input data structures
			wpPrintGpxIn()
			fmt.Printf("\nDisplay of input data structures completed.\n")
			
		case "12":
			// Print GPX solution data structures
			wpPrintGpxSoln()
			fmt.Printf("\nDisplay of solution completed.\n")

		case "13":
			// Print Cplex solution
			wpPrintCplexSoln()
									
		default:

			// If the command was not present in this wrapper, check the other ones.
			// Only if the command cannot be satisfied by any of the secondary
			// wrappers treat this as an "error" and display the available commands.
						
			if lpoMenuOn {
				if err = runLpoWrapper(cmdOption); err == nil {
					// Found the command in gpx menu, continue
					continue
				}
			}

			if gpxMenuOn {
				if err = runGpxWrapper(cmdOption); err == nil {
					// Found the command in gpx menu, continue
					continue
				}
			}

			fmt.Printf("Unsupported option: '%s'\n", cmdOption)
			printOptions()
						
		} // end of switch on cmdOption
	} // end for looping over commands

}

//==============================================================================

// main function calls the main wrapper. It accepts no arguments and returns
// no values.
func main() {
	
	runMainWrapper()
}