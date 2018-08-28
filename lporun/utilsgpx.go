// // +build exclude

//==============================================================================
// This file contains functions which depend on the presence of gpx.
// 01    Jul. 4, 2018  First version, uploaded to github
// 02   Aug. 28, 2018  Simplified to reduce complexity and remove functionality


package main

import (
	"fmt"
	"github.com/go-opt/gpx"
	"github.com/go-opt/lpo"
	"github.com/pkg/errors"
	"time"
)

// Need to declare gpx variables here to avoid passing them as arguments to the
// wrapper functions as individual wrapper commands are executed.

var gName     string            // gpx input problem name
var gRows   []gpx.InputRow      // gpx input rows
var gCols   []gpx.InputCol      // gpx input cols
var gElem   []gpx.InputElem     // gpx input elems
var gObj    []gpx.InputObjCoef  // gpx input objective function coefficients
var sObjVal   float64           // Solution value of objective function
var sRows   []gpx.SolnRow       // Solution rows provided by gpx
var sCols   []gpx.SolnCol       // Solution columns provided by gpx

//==============================================================================

// wpCplexSolveProb illustrates an example of a problem solved using the internal
// data structures. It reads data from file, populates the internal data structures,
// solves the problem, prints the solution, and gives user the option to save
// the model and solution to file. Function accepts no arguments.
// In case of failure, function returns an error.
func wpCplexSolveProb() error {
	var userString          string  // holder for general input from user
	var psCtrl          lpo.PsCtrl  // control structure for reductions
	var err                  error  // error received from called functions

	fmt.Printf("\nThis example illustrates how to read the model definition from an\n")
	fmt.Printf("MPS file, reduce the problem size, solve it via Cplex, and display\n")
	fmt.Printf("the results.\n\n")
	
	// In a previous incarnation, all values were provided by the user.
	// Now they are hard-coded, so populate the control structure directly.

	psCtrl.DelRowNonbinding  = true
	psCtrl.DelRowSingleton   = true
	psCtrl.DelColSingleton   = true
	psCtrl.DelFixedVars      = true
	psCtrl.RunSolver         = true
	psCtrl.MaxIter           = 10
	psCtrl.FileInMps         = inputLg
	psCtrl.FileOutSoln       = outSoln
	psCtrl.FileOutPsop       = outPsop
	psCtrl.FileOutMpsRdcd    = outRedMtx

	startTime := time.Now()					
	err = lpo.CplexSolveProb(psCtrl, &psResult)
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

