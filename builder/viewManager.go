package builder

/*


######ComponentTree
ComponentTree finds all views and constructs either a View or Layout object representing the file
It also builds relationships between Layouts to Layouts and Views to Layouts

It can also return the correct parent layout that applies to the view or layout

New methods needed which operate on both Layouts and Components:

AddSvelteFile(Path)
RemoveSvelteFile(Path)

A rename is just the above two since things like applicable layout files could have changed.



#####View manager
View manager holds a record of all views

It triggers views scan by using ComponentTree
It triggers SSR and Browser builds


It renders the view when requested



*/

type viewManager struct {
}
