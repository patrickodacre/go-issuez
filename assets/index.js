import './scss/index.scss'
import 'bootstrap'
const feather = require('feather-icons')
import resources from './resources'
import projectPageModule from './projectPageModule'
import featurePageModule from './featurePageModule'

window.onload = () => {

    resources()

    projectPageModule()
    featurePageModule()
}
